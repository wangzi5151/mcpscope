package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wangzi5151/mcpscope/internal/history"
	"github.com/wangzi5151/mcpscope/internal/mcp"
)

func TestFirstLine(t *testing.T) {
	if got := firstLine("a\nb\nc"); got != "a" {
		t.Fatalf("got %q", got)
	}
	if got := firstLine("single"); got != "single" {
		t.Fatalf("got %q", got)
	}
}

func TestArgsTemplate(t *testing.T) {
	if got := argsTemplate(nil); got != "{}" {
		t.Fatalf("nil schema: got %q", got)
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":   map[string]any{"type": "string", "example": "bob"},
			"count":  map[string]any{"type": "integer", "default": 3},
			"flag":   map[string]any{"type": "boolean"},
			"tags":   map[string]any{"type": "array"},
			"nested": map[string]any{"type": "object"},
			"free":   map[string]any{},
		},
	}
	got := argsTemplate(schema)
	for _, want := range []string{`"name": "bob"`, `"count": 3`, `"flag": false`, `"tags": []`, `"nested": {}`, `"free": ""`} {
		if !strings.Contains(got, want) {
			t.Fatalf("template missing %s in:\n%s", want, got)
		}
	}
}

func TestRenderResult(t *testing.T) {
	// Plain text.
	r := &mcp.CallResult{Content: []mcp.ContentBlock{{Type: "text", Text: "hello"}}}
	if got := renderResult(r); !strings.Contains(got, "hello") {
		t.Fatalf("got %q", got)
	}
	// Tool-side error is surfaced.
	r = &mcp.CallResult{IsError: true, Content: []mcp.ContentBlock{{Type: "text", Text: "boom"}}}
	if got := renderResult(r); !strings.Contains(got, "boom") {
		t.Fatalf("got %q", got)
	}
	// Non-text blocks are rendered as summaries, not dropped silently.
	r = &mcp.CallResult{Content: []mcp.ContentBlock{
		{Type: "text", Text: "see image"},
		{Type: "image", MimeType: "image/png"},
		{Type: "resource", URI: "file:///tmp/a.txt"},
	}}
	got := renderResult(r)
	for _, want := range []string{"see image", "[image: image/png]", "file:///tmp/a.txt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderResult missing %q in %q", want, got)
		}
	}
	// Empty result.
	r = &mcp.CallResult{}
	if got := renderResult(r); !strings.Contains(got, "empty result") {
		t.Fatalf("got %q", got)
	}
}

func TestPrettyJSON(t *testing.T) {
	if got := prettyJSON(nil); !strings.Contains(got, "none") {
		t.Fatalf("got %q", got)
	}
	if got := prettyJSON(map[string]any{"a": 1}); !strings.Contains(got, `"a": 1`) {
		t.Fatalf("got %q", got)
	}
}

func TestHistoryItemTitle(t *testing.T) {
	e := historyEntry{Tool: "add"}
	it := historyItem{e: e}
	if !strings.Contains(it.Title(), "add") || !strings.Contains(it.Title(), "ok") {
		t.Fatalf("got %q", it.Title())
	}
	e.IsError = true
	it = historyItem{e: e}
	if !strings.Contains(it.Title(), "err") {
		t.Fatalf("got %q", it.Title())
	}
}

func TestNewModelLoadsPersistedHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv(history.EnvPath, path)
	entry := history.Entry{At: time.Now(), Server: "mock 1", Tool: "add", ArgsJSON: `{"a":1}`, Output: "2"}
	if err := history.Append(path, entry); err != nil {
		t.Fatal(err)
	}
	m := NewModel(nil, "mock", "1", "")
	if len(m.history) != 1 || m.history[0].Tool != "add" {
		t.Fatalf("history not loaded: %+v", m.history)
	}
	if m.historyPath != path {
		t.Fatalf("historyPath = %q, want %q", m.historyPath, path)
	}
}
