# ◉ mcpscope — terminal MCP server debugger

[![CI](https://github.com/wangzi5151/mcpscope/actions/workflows/ci.yml/badge.svg)](https://github.com/wangzi5151/mcpscope/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/wangzi5151/mcpscope)](https://github.com/wangzi5151/mcpscope/releases)
[![Go](https://img.shields.io/badge/go-1.23+-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Debug any [Model Context Protocol](https://modelcontextprotocol.io) server from your terminal —
like Postman, but for MCP, and right where you already work (including over SSH).

Browse tools, resources and prompts, call tools with JSON arguments, and review
every call in history. No browser, no Electron, one static binary.

```
mcpscope -- npx -y @modelcontextprotocol/server-filesystem /tmp
```

<details>
<summary>What it looks like (actual terminal capture)</summary>

```
 ◉ mcpscope   server-filesystem 1.0.0
  1 Tools     2 Resources     3 Prompts     4 History
   Tools         ╭─────────────────────────────────────╮
                 │ read_file                           │
│ read_file      │                                     │
│ Read a file    │ Read a file from the filesystem     │
                 │                                     │
  write_file     │ Input schema                        │
                 │ {                                   │
                 │   "properties": {                   │
                 │     "path": { "type": "string" },   │
                 │     ...                             │
                 │   }                                 │
                 │ }                                   │
╰─────────────────────────────────────────────────────╯
 tab switch tab   ↑/↓ j/k navigate   / filter   enter call tool   q quit
```

</details>

## Features

- 🔌 **Any stdio MCP server** — spawns your server as a child process and speaks
  newline-delimited JSON-RPC 2.0 (MCP `2024-11-05`)
- 🧰 **Tools tab** — list tools, inspect input schemas, call with a pre-filled
  JSON template (built from the schema's examples/defaults)
- 📦 **Resources & Prompts tabs** — browse everything the server advertises
- 🕘 **History tab** — every call with timestamp, arguments and output
- 🤖 **Scriptable** — `--list-tools` dumps the tool catalogue as JSON for CI/pipes
- 🖥️ **Single static binary** — Linux, macOS, Windows (amd64/arm64)

## Install

**Prebuilt binaries** — download from [Releases](https://github.com/wangzi5151/mcpscope/releases):

```sh
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/wangzi5151/mcpscope/main/install.sh | bash
```

**From source** (Go 1.23+):

```sh
go install github.com/wangzi5151/mcpscope/cmd/mcpscope@latest
```

**Build it yourself:**

```sh
git clone https://github.com/wangzi5151/mcpscope && cd mcpscope
make build        # -> ./mcpscope
```

## Usage

```sh
# Interactive TUI
mcpscope -- <server-command> [args...]

# Examples
mcpscope -- npx -y @modelcontextprotocol/server-filesystem /tmp
mcpscope -- python3 my_mcp_server.py
mcpscope --timeout 10s -- ./target/debug/my-server

# Non-interactive: print tools as JSON (great for scripts)
mcpscope --list-tools -- python3 my_mcp_server.py | jq '.[].name'

# Non-interactive: call a tool directly
mcpscope --call add --args '{"a": 40, "b": 2}' -- python3 my_mcp_server.py
```

See [docs/USAGE.md](docs/USAGE.md) for the full guide.

### Keybindings

| Key | Action |
|---|---|
| `tab` / `1–4` | Switch tab (Tools / Resources / Prompts / History) |
| `↑↓` `j` `k` | Navigate |
| `/` | Filter list |
| `enter` | Call selected tool / read resource / render prompt / view history entry |
| `r` | Re-run selected history entry |
| `i` | Server info |
| `ctrl+s` | Run the call (in the argument editor) |
| `esc` | Back / cancel |
| `q` | Quit |

## How it works

```
┌───────────┐   stdio (JSON-RPC 2.0)   ┌────────────────┐
│ mcpscope  │ ◄──────────────────────► │ your MCP server │
│  (TUI)    │  initialize → tools/list │  (any language) │
└───────────┘  tools/call → result     └────────────────┘
```

mcpscope spawns your server, performs the MCP `initialize` handshake, then
proxies `tools/list`, `tools/call`, `resources/list` and `prompts/list`.
The server's stderr is left alone for its own logging.

## FAQ

**Which MCP servers work?** Any server speaking the stdio transport with
protocol version `2024-11-05` (or compatible). SSE/HTTP transports are not
supported yet — see [roadmap](docs/USAGE.md#roadmap).

**The TUI says "no TTY detected"?** mcpscope needs an interactive terminal.
In scripts/CI use `--list-tools` instead.

**A tool call hangs?** Tune it with `--timeout 10s` (default `30s`).

## Roadmap

- SSE / streamable-HTTP transports
- Save/load named server profiles
- Export history as Markdown

## License

MIT — see [LICENSE](LICENSE).
