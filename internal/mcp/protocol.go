// Package mcp implements a minimal Model Context Protocol (MCP) client
// speaking JSON-RPC 2.0 over a child process' stdio, as specified by the
// MCP stdio transport (newline-delimited JSON-RPC messages).
//
// Supported protocol version: 2024-11-05.
package mcp

import "encoding/json"

// ProtocolVersion is the MCP protocol version this client speaks.
const ProtocolVersion = "2024-11-05"

// JSON-RPC 2.0 envelope types.

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return e.Message }

// MCP handshake types.

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type clientCapabilities struct{}

type initializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    clientCapabilities `json:"capabilities"`
	ClientInfo      clientInfo         `json:"clientInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	Tools     *struct{} `json:"tools,omitempty"`
	Resources *struct{} `json:"resources,omitempty"`
	Prompts   *struct{} `json:"prompts,omitempty"`
	Logging   *struct{} `json:"logging,omitempty"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type listToolsResult struct {
	Tools []Tool `json:"tools"`
}

// ContentBlock is a single block inside a tool result.
type ContentBlock struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	Data     string         `json:"data,omitempty"`
	MimeType string         `json:"mimeType,omitempty"`
	Extra    map[string]any `json:"-"`
}

// CallResult is the result of tools/call.
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// Text concatenates all text blocks, the most useful rendering for a TUI.
func (r *CallResult) Text() string {
	out := ""
	for _, b := range r.Content {
		if b.Type == "text" {
			if out != "" {
				out += "\n"
			}
			out += b.Text
		}
	}
	return out
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Resource describes an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type listResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// ResourceContent is one content item returned by resources/read.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type readResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// Prompt describes an MCP prompt.
type Prompt struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type listPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptMessage is a single message in a rendered prompt.
type PromptMessage struct {
	Role    string       `json:"role"`
	Content ContentBlock `json:"content"`
}

type getPromptResult struct {
	Messages []PromptMessage `json:"messages"`
}
