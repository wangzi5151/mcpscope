package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wangzi5151/mcpscope/internal/history"
)

func TestRunVersion(t *testing.T) {
	if err := run([]string{"--version"}); err != nil {
		t.Fatalf("run --version: %v", err)
	}
}

func TestRunMissingCommand(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("expected an error when no server command is given")
	}
}

func TestRunExportHistoryHonoursHistoryFile(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history.jsonl")
	entry := history.Entry{
		At:   time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		Tool: "echo", ArgsJSON: `{"message":"hi"}`, Output: "hi",
	}
	if err := history.Append(histPath, entry); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "export.json")
	err := run([]string{"--export-history", dest, "--history-file", histPath})
	if err != nil {
		t.Fatalf("run --export-history: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"tool": "echo"`) {
		t.Fatalf("export did not come from --history-file:\n%s", b)
	}
}

func TestRunExportHistoryBadFormat(t *testing.T) {
	dir := t.TempDir()
	err := run([]string{
		"--export-history", filepath.Join(dir, "x.out"),
		"--history-file", filepath.Join(dir, "missing.jsonl"),
		"--history-format", "yaml",
	})
	if err == nil {
		t.Fatal("expected an error for an unsupported --history-format")
	}
}
