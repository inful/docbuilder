package commands

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// DaemonCmd implements the 'daemon' command.
type DaemonCmd struct {
	DataDir string `short:"d" default:"./daemon-data" help:"Data directory for daemon state"`
}

func (d *DaemonCmd) Run(_ *Global, root *CLI) error {
	// Load .env file if it exists (before config)
	if err := LoadEnvFile(); err == nil {
		slog.Debug("Loaded environment variables from .env file")
	}

	result, cfg, err := config.LoadWithResult(root.Config)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryConfig, "load config").Build()
	}
	// Print any normalization warnings
	for _, w := range result.Warnings {
		slog.Warn(w)
	}
	return RunDaemon(cfg, d.DataDir, root.Config)
}

func RunDaemon(cfg *config.Config, dataDir, configPath string) error {
	slog.Info("Starting daemon mode", "data_dir", dataDir)

	// Create main context for the daemon
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create and start the daemon with config file watching
	d, err := daemon.NewDaemonWithConfigFile(cfg, configPath)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "failed to create daemon").Build()
	}

	// Start daemon in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	slog.Info("Daemon started, waiting for shutdown signal...")

	// Wait for either error or shutdown signal
	select {
	case err := <-errChan:
		if err != nil {
			return derrors.WrapError(err, derrors.CategoryInternal, "daemon error").Build()
		}
	case <-ctx.Done():
		slog.Info("Shutdown signal received, stopping daemon...")
	}

	// Stop daemon gracefully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()

	if err := d.Stop(stopCtx); err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "failed to stop daemon").Build()
	}

	slog.Info("Daemon stopped successfully")
	return nil
}
