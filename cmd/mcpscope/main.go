// Command mcpscope is a terminal MCP server debugger: connect to any MCP
// server over stdio, browse its tools/resources/prompts, call tools with
// JSON arguments, and review call history.
//
// Usage:
//
//	mcpscope [--timeout 30s] [--list-tools] -- <command> [args...]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/wangzi5151/mcpscope/internal/mcp"
	"github.com/wangzi5151/mcpscope/internal/tui"
)

var (
	version = "dev" // overridden by -ldflags
	commit  = "none"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mcpscope: "+err.Error())
		os.Exit(1)
	}
}

func run(argv []string) error {
	fs := flag.NewFlagSet("mcpscope", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 30*time.Second, "per-request timeout, e.g. 10s, 1m")
	showVersion := fs.Bool("version", false, "print version and exit")
	listTools := fs.Bool("list-tools", false, "non-interactive: print the server's tools as JSON and exit")
	callTool := fs.String("call", "", "non-interactive: call a tool by name and print its result")
	callArgs := fs.String("args", "{}", "JSON arguments for --call, e.g. '{\"a\": 1}'")
	fs.SetOutput(os.Stderr)
	// Custom usage so the "-- <command>" shape is obvious.
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "mcpscope — terminal MCP server debugger")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  mcpscope [--timeout 30s] -- <command> [args...]")
		fmt.Fprintln(os.Stderr, "  mcpscope --list-tools -- <command> [args...]")
		fmt.Fprintln(os.Stderr, "  mcpscope --call <tool> [--args '{...}'] -- <command> [args...]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  mcpscope -- npx -y @modelcontextprotocol/server-filesystem /tmp")
		fmt.Fprintln(os.Stderr, "  mcpscope --list-tools -- python3 my_server.py")
		fmt.Fprintln(os.Stderr, "  mcpscope --call add --args '{\"a\": 40, \"b\": 2}' -- python3 my_server.py")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // -h already printed usage
		}
		return err
	}
	if *showVersion {
		fmt.Printf("mcpscope %s (commit %s)\n", version, commit)
		return nil
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		return fmt.Errorf("missing server command after --")
	}
	command, args := rest[0], rest[1:]

	ctx := context.Background()
	client, err := mcp.Dial(ctx, command, args, *timeout)
	if err != nil {
		return fmt.Errorf("could not connect: %w", err)
	}
	defer client.Close()

	if *listTools {
		return dumpTools(ctx, client)
	}

	if *callTool != "" {
		return callOnce(ctx, client, *callTool, *callArgs)
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("no TTY detected: mcpscope needs an interactive terminal (or use --list-tools)")
	}
	name, ver, instr := client.ServerInfo()
	return tui.Run(client, name, ver, instr)
}
// dumpTools implements --list-tools: print tools as indented JSON.
func dumpTools(ctx context.Context, client *mcp.Client) error {
	tools, err := client.ListTools(ctx)
	if err != nil {
		return err
	}
	if tools == nil {
		tools = []mcp.Tool{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tools)
}

// callOnce implements --call: invoke one tool and print its text output.
// Exits non-zero when the tool itself reports an error.
func callOnce(ctx context.Context, client *mcp.Client, name, argsJSON string) error {
	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Errorf("invalid --args JSON: %w", err)
		}
	}
	res, err := client.CallTool(ctx, name, args)
	if err != nil {
		return err
	}
	fmt.Println(res.Text())
	if res.IsError {
		return fmt.Errorf("tool %q returned an error", name)
	}
	return nil
}
