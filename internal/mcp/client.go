package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DefaultTimeout bounds every request round-trip.
const DefaultTimeout = 30 * time.Second

// Client is a high-level MCP client over a stdio transport.
type Client struct {
	t        *stdioTransport
	timeout  time.Duration
	server   initializeResult
	handlers map[string]func(json.RawMessage)
}

// Dial spawns command with args and performs the MCP initialize handshake.
func Dial(ctx context.Context, command string, args []string, timeout time.Duration) (*Client, error) {
	return DialWithInfo(ctx, command, args, timeout, "mcpscope", version())
}

// version is overridden at link time via -ldflags.
var appVersion = "dev"

func version() string { return appVersion }

// DialWithInfo is Dial with an explicit client name/version.
func DialWithInfo(ctx context.Context, command string, args []string, timeout time.Duration, name, ver string) (*Client, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	c := &Client{timeout: timeout, handlers: make(map[string]func(json.RawMessage))}
	t, err := newStdioTransport(ctx, command, args, c.onNotify)
	if err != nil {
		return nil, err
	}
	c.t = t

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := t.call(reqCtx, "initialize", initializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientInfo:      clientInfo{Name: name, Version: ver},
	})
	if err != nil {
		_ = t.close()
		return nil, fmt.Errorf("mcp: initialize: %w", err)
	}
	var initRes initializeResult
	if err := json.Unmarshal(resp.Result, &initRes); err != nil {
		_ = t.close()
		return nil, fmt.Errorf("mcp: decode initialize result: %w", err)
	}
	c.server = initRes
	// Best effort; some servers don't require it.
	_ = t.notifyOne("notifications/initialized", struct{}{})
	return c, nil
}

// OnNotification registers a handler for a server->client notification method.
func (c *Client) OnNotification(method string, fn func(json.RawMessage)) {
	c.handlers[method] = fn
}

func (c *Client) onNotify(method string, params json.RawMessage) {
	if fn, ok := c.handlers[method]; ok && fn != nil {
		fn(params)
	}
}

// ServerInfo returns the server's self-description from the handshake.
func (c *Client) ServerInfo() (name, ver, instructions string) {
	return c.server.ServerInfo.Name, c.server.ServerInfo.Version, c.server.Instructions
}

// Capabilities reports which feature groups the server advertised.
func (c *Client) Capabilities() (tools, resources, prompts bool) {
	return c.server.Capabilities.Tools != nil,
		c.server.Capabilities.Resources != nil,
		c.server.Capabilities.Prompts != nil
}

func (c *Client) roundTrip(ctx context.Context, method string, params any, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.t.call(reqCtx, method, params)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("mcp: decode %s result: %w", method, err)
	}
	return nil
}

// ListTools returns the server's tool catalogue.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var r listToolsResult
	if err := c.roundTrip(ctx, "tools/list", struct{}{}, &r); err != nil {
		return nil, err
	}
	return r.Tools, nil
}

// CallTool invokes a tool by name with JSON arguments.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	var r CallResult
	if err := c.roundTrip(ctx, "tools/call", callToolParams{Name: name, Arguments: args}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListResources returns the server's resource catalogue.
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	var r listResourcesResult
	if err := c.roundTrip(ctx, "resources/list", struct{}{}, &r); err != nil {
		return nil, err
	}
	return r.Resources, nil
}

// ListPrompts returns the server's prompt catalogue.
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	var r listPromptsResult
	if err := c.roundTrip(ctx, "prompts/list", struct{}{}, &r); err != nil {
		return nil, err
	}
	return r.Prompts, nil
}

// ReadResource reads a resource's content by URI.
func (c *Client) ReadResource(ctx context.Context, uri string) ([]ResourceContent, error) {
	var r readResourceResult
	if err := c.roundTrip(ctx, "resources/read", map[string]any{"uri": uri}, &r); err != nil {
		return nil, err
	}
	return r.Contents, nil
}

// GetPrompt renders a prompt by name with optional arguments.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]any) ([]PromptMessage, error) {
	var r getPromptResult
	params := map[string]any{"name": name}
	if len(args) > 0 {
		params["arguments"] = args
	}
	if err := c.roundTrip(ctx, "prompts/get", params, &r); err != nil {
		return nil, err
	}
	return r.Messages, nil
}

// Close shuts down the transport and reaps the child process.
func (c *Client) Close() error { return c.t.close() }
