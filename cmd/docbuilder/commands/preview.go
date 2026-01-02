package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon"
)

const configVersion = "2.0"

// PreviewCmd starts a local server watching a docs directory without forge polling.
type PreviewCmd struct {
	DocsDir        string `short:"d" name:"docs-dir" default:"./docs" help:"Path to local docs directory to watch."`
	OutputDir      string `short:"o" name:"output" default:"" help:"Output directory for the generated site (defaults to temp)."`
	Theme          string `name:"theme" default:"relearn" help:"Hugo theme to use (hextra, docsy, or relearn)."`
	Title          string `name:"title" default:"Local Preview" help:"Site title."`
	BaseURL        string `name:"base-url" default:"http://localhost:1316" help:"Base URL used in Hugo config."`
	Port           int    `name:"port" default:"1316" help:"Docs server port."`
	LiveReloadPort int    `name:"livereload-port" default:"0" help:"LiveReload server port (defaults to port+3)."`
	NoLiveReload   bool   `name:"no-live-reload" help:"Disable LiveReload SSE and script injection for preview."`
}

//nolint:forbidigo // fmt is used for user-facing messages
func (p *PreviewCmd) Run(_ *Global, _ *CLI) error {
	// Setup signal-based context for graceful shutdown
	sigctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	// Build a minimal in-memory config
	cfg := &config.Config{}
	cfg.Version = configVersion

	// Initialize monitoring config with defaults
	cfg.Monitoring = &config.MonitoringConfig{
		Health: config.MonitoringHealth{
			Path: "/health",
		},
		Metrics: config.MonitoringMetrics{
			Enabled: false,
			Path:    "/metrics",
		},
	}

	// Initialize daemon config
	liveReloadPort := p.LiveReloadPort
	if liveReloadPort == 0 {
		liveReloadPort = p.Port + liveReloadPortOffset
	}
	cfg.Daemon = &config.DaemonConfig{
		HTTP: config.HTTPConfig{
			DocsPort:       p.Port,
			WebhookPort:    p.Port + webhookPortOffset,
			AdminPort:      p.Port + adminPortOffset,
			LiveReloadPort: liveReloadPort,
		},
	}

	// If no output provided, create a temporary directory
	outDir := p.OutputDir
	tempOut := ""
	if outDir == "" {
		tmp, err := os.MkdirTemp("", "docbuilder-preview-*")
		if err != nil {
			return fmt.Errorf("create temp output: %w", err)
		}
		outDir = tmp
		tempOut = tmp
		slog.Info("Using temporary output directory for preview", "output", outDir)
		fmt.Println("Preview output directory:", outDir)
	}
	cfg.Output.Directory = outDir
	cfg.Output.Clean = true
	cfg.Hugo.Title = p.Title
	cfg.Hugo.Description = "DocBuilder local preview"
	cfg.Hugo.BaseURL = p.BaseURL
	cfg.Build.RenderMode = config.RenderModeAlways
	// Enable LiveReload by default for preview, unless explicitly disabled.
	cfg.Build.LiveReload = !p.NoLiveReload

	// Single local repository entry pointing to DocsDir
	cfg.Repositories = []config.Repository{{
		URL:    p.DocsDir,
		Name:   "local",
		Branch: "",
		Paths:  []string{"."},
	}}

	return daemon.StartLocalPreview(sigctx, cfg, p.Port, tempOut)
}
