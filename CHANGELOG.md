# Changelog

All notable changes to mcpscope are documented here.

## Unreleased

### Added

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
