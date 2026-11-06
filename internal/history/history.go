// Package history persists mcpscope tool-call history as JSONL and exports it
// as Markdown or JSON.
package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnvPath overrides the default history file location (useful for tests and
// portable setups).
const EnvPath = "MCPSCOPE_HISTORY"

// Entry is one recorded tool call.
type Entry struct {
	At       time.Time `json:"at"`
	Server   string    `json:"server,omitempty"`
	Tool     string    `json:"tool"`
	ArgsJSON string    `json:"args"`
	Output   string    `json:"output"`
	IsError  bool      `json:"isError,omitempty"`
}

// Path returns the JSONL history path, normally under the user config dir.
func Path() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvPath)); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("locate history directory: %w", herr)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "mcpscope", "history.jsonl"), nil
}

// Load reads all entries from path. A missing file yields an empty history.
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history %q: %w", path, err)
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("parse history %q line %d: %w", path, line, err)
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read history %q: %w", path, err)
	}
	return entries, nil
}

// Append adds one entry to the JSONL file, creating parent directories as
// needed.
func Append(path string, e Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open history %q: %w", path, err)
	}
	if err := json.NewEncoder(f).Encode(e); err != nil {
		f.Close()
		return fmt.Errorf("write history %q: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close history %q: %w", path, err)
	}
	return nil
}

// NormalizeFormat returns "json" or "markdown" for a user-supplied format.
func NormalizeFormat(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json", nil
	case "md", "markdown":
		return "markdown", nil
	default:
		return "", fmt.Errorf("unknown history format %q (use json or markdown)", format)
	}
}

// FormatForPath infers an export format from a file extension, defaulting to
// Markdown for stdout and unknown extensions.
func FormatForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return "json"
	default:
		return "markdown"
	}
}

// Export writes entries to w in the requested format.
func Export(w io.Writer, entries []Entry, format string) error {
	f, err := NormalizeFormat(format)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []Entry{}
	}
	switch f {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			return fmt.Errorf("encode history JSON: %w", err)
		}
		return nil
	default:
		return exportMarkdown(w, entries)
	}
}

func exportMarkdown(w io.Writer, entries []Entry) error {
	if _, err := fmt.Fprintf(w, "# mcpscope call history\n\n_%d call(s)_\n\n", len(entries)); err != nil {
		return err
	}
	for _, e := range entries {
		status := "ok"
		if e.IsError {
			status = "error"
		}
		if _, err := fmt.Fprintf(w, "## %s — `%s` (%s)\n\n", e.At.Format(time.RFC3339), e.Tool, status); err != nil {
			return err
		}
		if e.Server != "" {
			if _, err := fmt.Fprintf(w, "- **Server:** %s\n", e.Server); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "- **Time:** %s\n\n", e.At.Format(time.RFC3339)); err != nil {
			return err
		}
		args := strings.TrimSpace(e.ArgsJSON)
		if args == "" {
			args = "{}"
		}
		if _, err := fmt.Fprintf(w, "**Arguments**\n\n```json\n%s\n```\n\n", args); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "**Output**\n\n```text\n%s\n```\n\n", strings.TrimRight(e.Output, "\n")); err != nil {
			return err
		}
	}
	return nil
}
