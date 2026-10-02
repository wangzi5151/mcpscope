package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// mockServerMain runs inside the re-executed test binary and speaks enough
// MCP to exercise the client: initialize, tools/list, tools/call,
// resources/list, prompts/list.
func mockServerMain() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		respond := func(result any) {
			b, _ := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustJSON(result)})
			out.Write(b)
			out.WriteByte('\n')
			out.Flush()
		}
		fail := func(code int, msg string) {
			b, _ := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: msg}})
			out.Write(b)
			out.WriteByte('\n')
			out.Flush()
		}
		switch req.Method {
		case "initialize":
			respond(initializeResult{
				ProtocolVersion: ProtocolVersion,
				Capabilities:    serverCapabilities{Tools: &struct{}{}, Resources: &struct{}{}, Prompts: &struct{}{}},
				ServerInfo:      serverInfo{Name: "mock", Version: "0.0.1"},
				Instructions:    "mock server",
			})
		case "tools/list":
			respond(listToolsResult{Tools: []Tool{
				{Name: "echo", Description: "Echoes input", InputSchema: map[string]any{"type": "object"}},
				{Name: "boom", Description: "Always errors"},
			}})
		case "tools/call":
			var p callToolParams
			raw, _ := json.Marshal(req.Params)
			_ = json.Unmarshal(raw, &p)
			if p.Name == "boom" {
				fail(-32000, "kaboom")
				continue
			}
			msg, _ := p.Arguments["message"].(string)
			respond(CallResult{Content: []ContentBlock{{Type: "text", Text: "echo:" + msg}}})
		case "resources/list":
			respond(listResourcesResult{Resources: []Resource{{URI: "mock://x", Name: "x"}}})
		case "resources/read":
			respond(readResourceResult{Contents: []ResourceContent{
				{URI: "mock://x", MimeType: "text/plain", Text: "file contents here"},
			}})
		case "prompts/list":
			respond(listPromptsResult{Prompts: []Prompt{{Name: "p1", Description: "first"}}})
		case "prompts/get":
			respond(getPromptResult{Messages: []PromptMessage{
				{Role: "user", Content: ContentBlock{Type: "text", Text: "review this"}},
			}})
		default:
			fail(-32601, "method not found")
		}
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestMain(m *testing.M) {
	if os.Getenv("MCPSCOPE_MOCK") == "1" {
		mockServerMain()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func dialMock(t *testing.T) *Client {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Re-exec the test binary; TestMain diverts into mockServerMain.
	os.Setenv("MCPSCOPE_MOCK", "1")
	defer os.Unsetenv("MCPSCOPE_MOCK")
	c, err := DialWithInfo(context.Background(), exe, nil, 5*time.Second, "test", "0")
	if err != nil {
		t.Fatalf("dial mock: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestInitialize(t *testing.T) {
	c := dialMock(t)
	name, ver, instr := c.ServerInfo()
	if name != "mock" || ver != "0.0.1" || instr != "mock server" {
		t.Fatalf("bad server info: %q %q %q", name, ver, instr)
	}
	tools, res, prompts := c.Capabilities()
	if !tools || !res || !prompts {
		t.Fatalf("capabilities not advertised: %v %v %v", tools, res, prompts)
	}
}

func TestListAndCall(t *testing.T) {
	c := dialMock(t)
	ctx := context.Background()
	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	res, err := c.CallTool(ctx, "echo", map[string]any{"message": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || res.Text() != "echo:hi" {
		t.Fatalf("unexpected call result: %+v", res)
	}
	if _, err := c.CallTool(ctx, "boom", nil); err == nil {
		t.Fatal("expected error from boom tool")
	} else if err.Error() != "kaboom" {
		t.Fatalf("unexpected rpc error: %v", err)
	}
}

func TestListResourcesPrompts(t *testing.T) {
	c := dialMock(t)
	ctx := context.Background()
	rs, err := c.ListResources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].URI != "mock://x" {
		t.Fatalf("unexpected resources: %+v", rs)
	}
	ps, err := c.ListPrompts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "p1" {
		t.Fatalf("unexpected prompts: %+v", ps)
	}
}

func TestDialFailure(t *testing.T) {
	ctx := context.Background()
	if _, err := Dial(ctx, "/nonexistent-binary-xyz", nil, 2*time.Second); err == nil {
		t.Fatal("expected dial error for missing binary")
	}
}

func TestReadResource(t *testing.T) {
	c := dialMock(t)
	contents, err := c.ReadResource(context.Background(), "mock://x")
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || contents[0].Text != "file contents here" {
		t.Fatalf("unexpected contents: %+v", contents)
	}
}

func TestGetPrompt(t *testing.T) {
	c := dialMock(t)
	msgs, err := c.GetPrompt(context.Background(), "p1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Role != "user" || msgs[0].Content.Text != "review this" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}
