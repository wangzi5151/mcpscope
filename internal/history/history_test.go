package history

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleEntries() []Entry {
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	return []Entry{
		{At: at, Server: "mock 1.0", Tool: "echo", ArgsJSON: `{"message":"hi"}`, Output: "hi"},
		{At: at.Add(time.Minute), Server: "mock 1.0", Tool: "boom", ArgsJSON: `{}`, Output: "kaboom", IsError: true},
	}
}

func TestPathFromEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.jsonl")
	t.Setenv(EnvPath, want)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestAppendLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "history.jsonl")
	entries := sampleEntries()
	for _, e := range entries {
		if err := Append(path, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(got), len(entries))
	}
	if got[0].Tool != "echo" || got[1].Tool != "boom" || !got[1].IsError {
		t.Fatalf("unexpected entries: %+v", got)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

func TestExportJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Export(&buf, sampleEntries(), "json"); err != nil {
		t.Fatal(err)
	}
	var got []Entry
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON export: %v\n%s", err, buf.String())
	}
	if len(got) != 2 || got[0].Tool != "echo" || got[1].Output != "kaboom" {
		t.Fatalf("unexpected JSON export: %+v", got)
	}
}

func TestExportMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := Export(&buf, sampleEntries(), "markdown"); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{"# mcpscope call history", "`echo` (ok)", "`boom` (error)", `"message":"hi"`, "hi", "kaboom"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Markdown export missing %q in:\n%s", want, got)
		}
	}
}

func TestNormalizeFormat(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"json", "json"},
		{"JSON", "json"},
		{"md", "markdown"},
		{"markdown", "markdown"},
	} {
		got, err := NormalizeFormat(tc.in)
		if err != nil {
			t.Fatalf("NormalizeFormat(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("NormalizeFormat(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if _, err := NormalizeFormat("yaml"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
