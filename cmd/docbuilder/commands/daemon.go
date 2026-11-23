package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon"
)

// DaemonCmd implements the 'daemon' command.
type DaemonCmd struct {
	// DataDir is the directory the daemon keeps its state in: the state
	// file, the event log and the remote-head cache. Empty means the
	// configuration decides (daemon.storage.state_dir).
	DataDir string `short:"d" default:"" help:"Data directory for daemon state (default: the state_dir of the configuration)"`
}

func (d *DaemonCmd) Run(_ *Global, root *CLI) error {
	// Load .env file if it exists (before config)
	if err := LoadEnvFile(); err == nil {
		slog.Debug("Loaded environment variables from .env file")
	}

	result, cfg, err := config.LoadWithResult(root.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// Print any normalization warnings
	for _, w := range result.Warnings {
		slog.Warn(w)
	}
	applyDataDir(cfg, d.DataDir)
	return RunDaemon(cfg, root.Config)
}

// applyDataDir lets the --data-dir flag decide where the daemon keeps its
// state. The flag wins over the configuration: the operator asked for it on
// the command line. An empty flag leaves the configuration alone, so a
// configuration with a state_dir keeps it.
func applyDataDir(cfg *config.Config, dataDir string) {
	if dataDir == "" || cfg == nil || cfg.Daemon == nil {
		return
	}
	cfg.Daemon.Storage.StateDir = dataDir
}

func RunDaemon(cfg *config.Config, configPath string) error {
	slog.Info("Starting daemon mode", "data_dir", cfg.Daemon.Storage.StateDir)

	// Create main context for the daemon
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create and start the daemon with config file watching
	d, err := daemon.NewDaemonWithConfigFile(cfg, configPath)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
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
			return fmt.Errorf("daemon error: %w", err)
		}
	case <-ctx.Done():
		slog.Info("Shutdown signal received, stopping daemon...")
	}

	// Stop daemon gracefully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()

	if err := d.Stop(stopCtx); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	slog.Info("Daemon stopped successfully")
	return nil
}
