// Command docbuilder-mcp runs an MCP (Model Context Protocol) server over stdio
// so LLM hosts (Claude Desktop, Cursor, VS Code, etc.) can drive doc maintenance
// tasks against docbuilder — listing templates, scaffolding new docs, linting,
// and editing files.
//
// It is intentionally a standalone binary (not a subcommand of `docbuilder`)
// because MCP hosts spawn one process per server and expect a tight stdio
// JSON-RPC surface. Pulling in Kong + the full CLI for that would just add
// startup latency.
//
// Usage:
//
//	docbuilder-mcp \
//	    --config config.yaml \
//	    --base-url https://docs.example.com \
//	    --docs-dir ./docs
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/version"
)

func main() {
	var (
		configPath = flag.String("config", "config.yaml", "Path to docbuilder config.yaml (optional)")
		baseURL    = flag.String("base-url", "", "Override template discovery base URL")
		docsDir    = flag.String("docs-dir", "", "Default docs directory (default: ./docs)")
		verbose    = flag.Bool("verbose", false, "Enable debug logging")
		showVer    = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Fprintln(os.Stdout, version.Version)
		return
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	cfg := serverConfig{
		ConfigPath: *configPath,
		BaseURL:    *baseURL,
		DocsDir:    *docsDir,
		Verbose:    *verbose,
		Version:    version.Version,
		Logger:     logger,
	}

	if err := run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "docbuilder-mcp: %v\n", err)
		os.Exit(1)
	}
}
