package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// serverConfig carries all options the MCP server needs to start up.
//
// Keep this small: anything that needs to be reachable from a tool handler
// (or resource handler) belongs on ctxValue below, not here.
type serverConfig struct {
	ConfigPath string // path to docbuilder config.yaml; empty means skip config load
	BaseURL    string // override for template discovery; empty means use env / config
	DocsDir    string // default docs root for create/update/lint; empty means ./docs
	Verbose    bool
	Version    string
	Logger     *slog.Logger
}

// serverState is the shared, per-server state passed to every tool/resource
// handler. It is wrapped in a context key so handlers can reach it without
// relying on package globals.
type serverState struct {
	cfg        *config.Config // nil if config failed to load or no path given
	cfgPath    string         // original --config value, for resource reporting
	baseURL    string         // resolved template base URL
	docsDir    string         // absolute, resolved docs directory
	logger     *slog.Logger
	version    string
}

// ctxKey is a private type so context.Value can't be accidentally collided.
type ctxKey int

const stateKey ctxKey = 1

// run is the binary's entry point: load config, register tools/resources,
// and serve over stdio until the host closes the connection or a signal arrives.
func run(ctx context.Context, cfg serverConfig) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Best-effort .env load — the CLI does this too; we mirror so an MCP
	// host pointing at a project directory behaves the same as `docbuilder mcp`.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		cfg.Logger.Warn("failed to load .env", "error", err)
	}

	state, err := buildServerState(cfg)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	srv := server.NewMCPServer(
		"docbuilder-mcp",
		state.version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	registerTools(srv, state)
	registerResources(srv, state)

	// ServeStdio installs SIGINT/SIGTERM handlers internally, so we don't
	// need to plumb our own. It blocks until the host disconnects.
	if err := server.ServeStdio(srv); err != nil {
		return fmt.Errorf("serve stdio: %w", err)
	}
	return nil
}

// buildServerState loads config (if a path was given and exists), resolves
// the template base URL via the same precedence the CLI uses, and makes the
// docs directory absolute.
func buildServerState(cfg serverConfig) (*serverState, error) {
	st := &serverState{
		cfgPath: cfg.ConfigPath,
		logger:  cfg.Logger,
		version: cfg.Version,
	}

	// Load config — never fatal; many tools (read_doc, lint, template ops)
	// are useful even with no config.
	if cfg.ConfigPath != "" {
		if _, err := os.Stat(cfg.ConfigPath); err == nil {
			result, loaded, err := config.LoadWithResult(cfg.ConfigPath)
			if err != nil {
				cfg.Logger.Warn("failed to load config; continuing with empty config",
					"path", cfg.ConfigPath, "error", err)
			} else {
				st.cfg = loaded
				for _, w := range result.Warnings {
					cfg.Logger.Warn("config warning", "warning", w)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			cfg.Logger.Warn("config path not accessible", "path", cfg.ConfigPath, "error", err)
		}
	}

	// Resolve template base URL using CLI precedence: flag > env > config.
	st.baseURL = cfg.BaseURL
	if st.baseURL == "" {
		st.baseURL = os.Getenv("DOCBUILDER_TEMPLATE_BASE_URL")
	}
	if st.baseURL == "" && st.cfg != nil && st.cfg.Hugo.BaseURL != "" {
		st.baseURL = st.cfg.Hugo.BaseURL
	}

	// Resolve docs directory — always absolute so containment checks work.
	docsDir := cfg.DocsDir
	if docsDir == "" {
		docsDir = "docs"
	}
	abs, err := filepath.Abs(docsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve docs dir %q: %w", docsDir, err)
	}
	st.docsDir = abs

	if cfg.Verbose {
		cfg.Logger.Debug("server state",
			"config_path", st.cfgPath,
			"has_config", st.cfg != nil,
			"base_url", st.baseURL,
			"docs_dir", st.docsDir,
		)
	}

	return st, nil
}

// stateFromContext is the canonical way tool/resource handlers reach server state.
func stateFromContext(ctx context.Context) *serverState {
	if s, ok := ctx.Value(stateKey).(*serverState); ok {
		return s
	}
	return nil
}

// stateMiddleware wraps every handler so it can pull *serverState out of the
// request context. mcp-go doesn't pass our context through by default, so we
// use a middleware to inject the state.
func stateMiddleware(state *serverState) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx = context.WithValue(ctx, stateKey, state)
			return next(ctx, req)
		}
	}
}
