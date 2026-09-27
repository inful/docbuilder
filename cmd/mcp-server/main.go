// Command docbuilder-mcp runs an MCP (Model Context Protocol) server over stdio
// so LLM hosts (Claude Desktop, Cursor, VS Code, etc.) can drive doc maintenance
// tasks against docbuilder — listing templates, scaffolding new docs, linting,
// and editing files.
//
// It is intentionally a standalone binary (not a subcommand of `docbuilder`)
// because MCP hosts spawn one process per server and expect a tight stdio
// JSON-RPC surface.
//
// Subcommands:
//
//	docbuilder-mcp                     start the server (default if no subcommand given;
//	                                  flags: --config, --base-url, --docs-dir, --verbose)
//	docbuilder-mcp init [tools...]     set up MCP config for one or more clients
//	                                  (opencode, vscode). Default: project-level config,
//	                                  --global for user-level. Auto-detects installed
//	                                  tools when no argument is given.
//	docbuilder-mcp print [tools...]    print the MCP config JSON for the named tools
//	                                  without writing anything to disk.
//	docbuilder-mcp path [tool]        show where the MCP config file would be written
//	                                  for the named tool (project-level by default,
//	                                  pass --global for user-level).
//
// Why a subcommand for setup when MCP servers are normally invoked with just
// flags: hosts spawn `docbuilder-mcp` (no subcommand) which Kong still
// dispatches to the default `serve` command. Existing host configs of the
// form `"command": "docbuilder-mcp", "args": ["--config", "..."]` keep
// working unchanged.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// CLI is the top-level Kong CLI for docbuilder-mcp.
//
// `Serve` is marked as the default command with `default:"withargs"` so that
// `docbuilder-mcp --config foo.yaml` (no subcommand, flags before any
// subcommand name) still launches the server — matching the pre-subcommand
// invocation pattern that MCP hosts rely on. `default:"withargs"` is the
// Kong incantation that allows flags to come BEFORE the default subcommand
// name; plain `default:"1"` only dispatches the default subcommand when no
// args are given at all, which would break the MCP host spawn pattern.
type CLI struct {
	Version customVersionFlag `name:"version" help:"Print version and exit"`

	Serve ServeCmd `cmd:"" name:"serve" default:"withargs" help:"Run the MCP server over stdio (default if no command given)"`
	Init  InitCmd  `cmd:"" name:"init" help:"Set up MCP config for one or more clients (opencode, vscode)"`
	Print PrintCmd `cmd:"" name:"print" help:"Print MCP config JSON for one or more tools without writing to disk"`
	Path  PathCmd  `cmd:"" name:"path" help:"Show where the MCP config file would be written for a tool"`
}

// customVersionFlag prints the ldflag-injected Version string from
// internal/version, which GoReleaser sets via -ldflags at build time.
type customVersionFlag bool

// BeforeApply runs before the rest of the CLI parses; if --version is in
// the args, print and exit before subcommand dispatch.
func (customVersionFlag) BeforeApply(_ *kong.Context) error {
	_, _ = fmt.Fprintln(os.Stdout, version.Version)
	os.Exit(0)
	return nil
}

func main() {
	cli := CLI{}
	parser, err := kong.New(
		&cli,
		kong.Name("docbuilder-mcp"),
		kong.Description("MCP server for the docbuilder documentation pipeline."),
		kong.UsageOnError(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docbuilder-mcp: %v\n", err)
		os.Exit(1)
	}
	kctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "docbuilder-mcp: %v\n", err)
		os.Exit(1)
	}
	if err := kctx.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "docbuilder-mcp: %v\n", err)
		os.Exit(1)
	}
}
