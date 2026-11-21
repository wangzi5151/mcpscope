# Changelog

All notable changes to mcpscope are documented here.

## Unreleased

### Fixed

- `--history-file` is now honoured by the interactive TUI, not just by
  `--export-history`; previously the TUI always used the default/env path
  despite the README documenting the flag.
- The MCP `initialize` handshake now reports the real build version
  (`-ldflags -X main.version`) instead of always reporting `dev`.
- Markdown history exports can no longer be broken by tool output that
  itself contains ` ``` ` fences: the export fence is lengthened past the
  longest backtick run in the content.
- Binary (`blob`) resources no longer render as empty in the TUI; they
  show a `[binary content: <mime>, <n> bytes]` summary. Prompt messages
  with non-text content are likewise summarised instead of being shown
  as a bare "(non-text content)" placeholder.
- Argument templates now honour the JSON Schema `examples` array (first
  entry) in addition to the singular `example`.

### Added

- Prompt arguments (`arguments` in `prompts/list`) are decoded and shown
  in the Prompts tab detail pane, with required arguments marked.
- `CONTRIBUTING.md` with build/test instructions, and tests for the
  `cmd/mcpscope` flag plumbing (`--version`, `--export-history` with
  `--history-file`, bad `--history-format`).
- Cursor pagination for `tools/list`, `resources/list`, and `prompts/list`.
  The client follows `nextCursor` until the server returns no further pages and
  detects repeated cursors instead of looping forever.
- Persistent call history in `<user config dir>/mcpscope/history.jsonl`, loaded
  on TUI startup and appended after each call. Override with
  `MCPSCOPE_HISTORY` or `--history-file`.
- History export via `--export-history FILE` with Markdown or JSON output
  (`--history-format`, or inferred from `.md`/`.json`; `-` writes to stdout).
- Tests for pagination, content-block rendering (including URI/MIME summaries),
  history persistence, and both export formats.

### Changed

- `--call` now prints every content block: text verbatim and non-text blocks as
  summaries instead of dropping images, audio, and embedded resources.
- TUI result rendering uses the same complete content rendering, so non-text
  blocks show their type, MIME type, and URI.
- README and usage guide document pagination, complete `--call` output, history
  persistence, and export. Removed the completed export item from the roadmap.
