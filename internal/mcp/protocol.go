// Package mcp implements a minimal Model Context Protocol (MCP) client
// speaking JSON-RPC 2.0 over a child process' stdio, as specified by the
// MCP stdio transport (newline-delimited JSON-RPC messages).
//
// Supported protocol version: 2024-11-05.
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

type listToolsParams struct {
	Cursor string `json:"cursor,omitempty"`
}

type listToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ContentBlock is a single block inside a tool result.
type ContentBlock struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	Data     string           `json:"data,omitempty"`
	MimeType string           `json:"mimeType,omitempty"`
	URI      string           `json:"uri,omitempty"`
	Name     string           `json:"name,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
	Extra    map[string]any   `json:"-"`
}

// Summary renders one content block as plain text. Text blocks keep their
// content; non-text blocks become a compact description so images, audio and
// embedded resources are never silently dropped.
func (b ContentBlock) Summary() string {
	switch strings.ToLower(b.Type) {
	case "text":
		return b.Text
	case "image", "audio":
		if b.MimeType != "" {
			return fmt.Sprintf("[%s: %s]", b.Type, b.MimeType)
		}
		return "[" + b.Type + "]"
	case "resource", "resource_link":
		uri, mime := b.URI, b.MimeType
		if b.Resource != nil {
			if uri == "" {
				uri = b.Resource.URI
			}
			if mime == "" {
				mime = b.Resource.MimeType
			}
		}
		var summary string
		switch {
		case uri != "" && mime != "":
			summary = fmt.Sprintf("[%s: %s (%s)]", b.Type, uri, mime)
		case uri != "":
			summary = fmt.Sprintf("[%s: %s]", b.Type, uri)
		case mime != "":
			summary = fmt.Sprintf("[%s: %s]", b.Type, mime)
		default:
			summary = "[" + b.Type + "]"
		}
		if b.Resource != nil && b.Resource.Text != "" {
			return summary + "\n" + b.Resource.Text
		}
		return summary
	default:
		if b.MimeType != "" {
			return fmt.Sprintf("[%s: %s]", b.Type, b.MimeType)
		}
		if b.URI != "" {
			return fmt.Sprintf("[%s: %s]", b.Type, b.URI)
		}
		if b.Type != "" {
			return "[" + b.Type + "]"
		}
		return ""
	}
}

// CallResult is the result of tools/call.
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// Render returns all content blocks as plain text: text blocks verbatim and
// non-text blocks as summaries (type, MIME type and URI where available).
func (r *CallResult) Render() string {
	var parts []string
	for _, b := range r.Content {
		if s := b.Summary(); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
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

type listResourcesParams struct {
	Cursor string `json:"cursor,omitempty"`
}

type listResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
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

type listPromptsParams struct {
	Cursor string `json:"cursor,omitempty"`
}

type listPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// PromptMessage is a single message in a rendered prompt.
type PromptMessage struct {
	Role    string       `json:"role"`
	Content ContentBlock `json:"content"`
}

type getPromptResult struct {
	Messages []PromptMessage `json:"messages"`
}
