# Contributing to mcpscope

Thanks for helping improve mcpscope — a terminal debugger for
[Model Context Protocol](https://modelcontextprotocol.io) servers.

## Development

You need Go 1.23 or newer.

```sh
git clone https://github.com/wangzi5151/mcpscope && cd mcpscope
make build   # -> ./mcpscope
make test    # go test ./... -race -count=1
make vet     # go vet ./...
make fmt     # lists files that need gofmt (CI requires a clean result)
```

Try it against any stdio MCP server, for example:

```sh
./mcpscope -- npx -y @modelcontextprotocol/server-filesystem /tmp
./mcpscope --list-tools -- python3 my_mcp_server.py
```

## Guidelines

- Keep changes focused and reviewable; no wholesale rewrites.
- Add or update tests for behaviour you change. The MCP client tests run
  against a mock server implemented in `internal/mcp/client_test.go` —
  extend it when you add protocol behaviour.
- Run `gofmt`, `go vet ./...`, and `go test ./...` before opening a PR;
  CI runs the same checks on Linux, macOS, and Windows.
- Don't add a third-party dependency unless it is essential — mcpscope's
  appeal is a single static binary with a small dependency tree.
- Document user-visible changes in `CHANGELOG.md` under *Unreleased* and,
  where relevant, in `README.md` / `docs/USAGE.md`.

## Reporting bugs

Open an issue with:

- your OS/arch and `mcpscope --version` output,
- the server command you ran (redact secrets),
- what you expected, what happened, and any error shown in the TUI footer
  or on stderr.
