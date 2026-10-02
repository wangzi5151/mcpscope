# mcpscope usage guide

## Connecting

mcpscope spawns your MCP server as a child process and talks to it over
stdin/stdout using newline-delimited JSON-RPC 2.0:

```sh
mcpscope -- <command> [args...]
```

The command can be anything executable: `npx`, `python3`, `node`, a compiled
binary, a shell script. Arguments after `--` are passed through untouched.

On startup mcpscope sends `initialize` (protocol version `2024-11-05`),
then `notifications/initialized`, then loads the tool/resource/prompt
catalogues in parallel.

## The interface

Four tabs, switched with `tab` or `1`–`4`:

### 1 · Tools

Left: the tool list (filter with `/`). Right: the selected tool's description
and pretty-printed input schema.

Press `enter` on a tool to call it. An editor opens with the arguments
pre-filled from the schema — each property gets its `example`, its `default`,
or a type-appropriate zero value (`""`, `0`, `false`, `[]`, `{}`), so most
calls only need small tweaks. Press `ctrl+s` to run, `esc` to cancel.

The result screen shows the concatenated text output (non-text blocks like
images are noted, not rendered). Tool-side errors are shown in red.

### 2 · Resources

Browse advertised resources: name, URI and MIME type. Press `enter` to read
the selected resource and view its content.

### 3 · Prompts

Browse advertised prompts with their descriptions. Press `enter` to render
the selected prompt and view its messages.

### 4 · History

Every tool call of the session, newest first, with timestamp, status
(ok/error), the exact arguments sent, and the full output. Press `enter` to
open an entry in the scrollable result view, or `r` to re-run it with the
same arguments pre-filled.

Press `i` anywhere in the browser to see the server's name, version,
protocol and instructions.

## Non-interactive mode

For scripts and CI, skip the TUI entirely:

```sh
# Dump the tool catalogue as JSON
mcpscope --list-tools -- python3 server.py

# Just the names
mcpscope --list-tools -- python3 server.py | jq -r '.[].name'

# Call a tool directly and print its text output
mcpscope --call add --args '{"a": 40, "b": 2}' -- python3 server.py
```

Exit code is non-zero if the server can't be reached or the handshake fails;
errors go to stderr.

## Timeouts

Each request (including tool calls) is bounded by `--timeout` (default `30s`):

```sh
mcpscope --timeout 10s -- ./my-server
```

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `could not connect: mcp: initialize: …` | the command failed to start, or exited immediately — try running it by hand first |
| `mcp: server closed the connection` | the server crashed or doesn't speak stdio JSON-RPC |
| `no TTY detected` | you're piping stdin — use `--list-tools`, or run in a real terminal |
| `tools/list: …` in red at the bottom | the server doesn't implement that method; other tabs still work |
| `invalid JSON arguments` | the editor content isn't valid JSON — fix and `ctrl+s` again |

## Roadmap

- SSE / streamable-HTTP transports (`--url`)
- Named server profiles (`~/.config/mcpscope/servers.yaml`)
- Export history to Markdown
