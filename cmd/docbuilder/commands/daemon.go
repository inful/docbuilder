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
	DataDir string `short:"d" help:"Data directory for daemon state" default:"./daemon-data"`
}

func (d *DaemonCmd) Run(_ *Global, root *CLI) error {
	cfg, err := config.Load(root.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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
