package main

import (
	"context"
	"log/slog"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// ServeCmd is the default subcommand. Running `docbuilder-mcp` with no
// subcommand lands here, preserving the pre-subcommand invocation pattern
// that MCP hosts rely on (`"command": "docbuilder-mcp", "args": ["--config", ...]`).
type ServeCmd struct {
	Config  string `short:"c" name:"config" default:"config.yaml" env:"DOCBUILDER_CONFIG" help:"Path to docbuilder config.yaml (optional)"`
	BaseURL string `name:"base-url" default:"" env:"DOCBUILDER_TEMPLATE_BASE_URL" help:"Override template discovery base URL"`
	DocsDir string `name:"docs-dir" default:"./docs" help:"Default docs directory (default: ./docs)"`
	Verbose bool   `short:"v" name:"verbose" env:"DOCBUILDER_VERBOSE" help:"Enable debug logging"`
}

// Run starts the MCP server. Mirrors the original main() behavior with
// flag-based parsing, just wired through Kong instead of the stdlib
// flag package.
func (s *ServeCmd) Run() error {
	level := slog.LevelInfo
	if s.Verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	cfg := serverConfig{
		ConfigPath: s.Config,
		BaseURL:    s.BaseURL,
		DocsDir:    s.DocsDir,
		Verbose:    s.Verbose,
		Version:    version.Version,
		Logger:     logger,
	}
	return run(context.Background(), cfg)
}
