package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// stdioTransport speaks newline-delimited JSON-RPC 2.0 with a child process:
// requests/notifications are written to its stdin (one JSON object per line),
// responses and server notifications are read from its stdout line by line.
// The child's stderr is discarded (servers use it for diagnostics).
type stdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner

	mu      sync.Mutex // guards writes + id allocation
	nextID  int64
	pending map[any]chan *rpcResponse

	// notify is invoked for server->client notifications (best effort).
	notify func(method string, params json.RawMessage)

	closed    chan struct{}
	closeOnce sync.Once
	writeErr  error
}

func newStdioTransport(ctx context.Context, command string, args []string, notify func(string, json.RawMessage)) (*stdioTransport, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdout pipe: %w", err)
	}
	// Keep stderr away from our stdout; servers log diagnostics there.
	if f, err := cmd.StderrPipe(); err == nil {
		go func() { _, _ = io.Copy(io.Discard, f) }()
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: start %q: %w", command, err)
	}
	t := &stdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		pending: make(map[any]chan *rpcResponse),
		notify:  notify,
		closed:  make(chan struct{}),
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	t.scanner = sc
	go t.readLoop()
	return t, nil
}

func (t *stdioTransport) readLoop() {
	defer t.failPending(fmt.Errorf("mcp: server closed the connection"))
	for t.scanner.Scan() {
		line := t.scanner.Bytes()
		// Skip blank lines; be liberal in what we accept.
		if len(line) == 0 {
			continue
		}
		var msg rpcResponse
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		// A message without an id is a server notification.
		if msg.ID == nil {
			var n rpcNotification
			if err := json.Unmarshal(line, &n); err == nil && t.notify != nil {
				t.notify(n.Method, mustRaw(n.Params))
			}
			continue
		}
		t.mu.Lock()
		ch, ok := t.pending[normID(msg.ID)]
		if ok {
			delete(t.pending, normID(msg.ID))
		}
		t.mu.Unlock()
		if ok {
			select {
			case ch <- &msg:
			case <-t.closed:
			}
		}
	}
}

func mustRaw(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

func (t *stdioTransport) failPending(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, ch := range t.pending {
		delete(t.pending, id)
		select {
		case ch <- &rpcResponse{Error: &rpcError{Code: -32000, Message: err.Error()}}:
		default:
		}
	}
}

// call sends a request and waits for its response or ctx cancellation.
func (t *stdioTransport) call(ctx context.Context, method string, params any) (*rpcResponse, error) {
	t.mu.Lock()
	if t.writeErr != nil {
		t.mu.Unlock()
		return nil, t.writeErr
	}
	t.nextID++
	id := t.nextID
	ch := make(chan *rpcResponse, 1)
	t.pending[id] = ch
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	line, err := json.Marshal(req)
	if err != nil {
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, err
	}
	line = append(line, '\n')
	if err := writeFull(t.stdin, line); err != nil {
		delete(t.pending, id)
		t.writeErr = fmt.Errorf("mcp: write to server: %w", err)
		t.mu.Unlock()
		return nil, t.writeErr
	}
	t.mu.Unlock()

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp, nil
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case <-t.closed:
		return nil, fmt.Errorf("mcp: transport closed")
	}
}

// notifyOne sends a JSON-RPC notification (no response expected).
func (t *stdioTransport) notifyOne(method string, params any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.writeErr != nil {
		return t.writeErr
	}
	n := rpcNotification{JSONRPC: "2.0", Method: method, Params: params}
	line, err := json.Marshal(n)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if err := writeFull(t.stdin, line); err != nil {
		t.writeErr = fmt.Errorf("mcp: write to server: %w", err)
		return t.writeErr
	}
	return nil
}

// writeFull loops until p is fully written; os.File.Write may write
// short on pipes, which would corrupt the JSON-RPC stream.
func writeFull(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		p = p[n:]
	}
	return nil
}

// key normalizes numeric JSON ids (float64) for map lookup.
func normID(id any) any {
	switch v := id.(type) {
	case float64:
		return int64(v)
	default:
		return v
	}
}

var _ = io.Discard

func (t *stdioTransport) close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		err = t.stdin.Close()
		// Give the child a chance to exit gracefully, then reap it.
		// A stubborn server that ignores the closed stdin must not hang
		// our own shutdown, so escalate to SIGKILL after a grace period.
		done := make(chan struct{})
		go func() { _ = t.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = t.cmd.Process.Kill()
			<-done
		}
	})
	t.failPending(fmt.Errorf("mcp: transport closed"))
	return err
}
