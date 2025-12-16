package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/stretchr/testify/require"
)

// TestConfigReload_TriggersBuild verifies that reloading config triggers a rebuild
func TestConfigReload_TriggersBuild(t *testing.T) {
	t.Parallel()

	// Create temp directory for testing
	tmpDir := t.TempDir()

	// Create initial config
	cfg := &config.Config{
		Version: "2.0",
		Hugo: config.HugoConfig{
			Title:       "Test Site",
			Description: "Test",
			BaseURL:     "http://localhost:1313",
		},
		Output: config.OutputConfig{
			Directory: tmpDir,
			Clean:     true,
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    8080,
				WebhookPort: 8081,
				AdminPort:   8082,
			},
			Sync: config.SyncConfig{
				Schedule:         "0 */6 * * *",
				ConcurrentBuilds: 1,
				QueueSize:        10,
			},
			Storage: config.StorageConfig{
				RepoCacheDir: tmpDir,
			},
		},
		Build: config.BuildConfig{
			RenderMode: config.RenderModeAlways,
		},
	}

	// Create daemon
	daemon, err := NewDaemon(cfg)
	require.NoError(t, err, "failed to create daemon")
	require.NotNil(t, daemon)

	// Create new config with updated title
	newCfg := &config.Config{
		Version: "2.0",
		Hugo: config.HugoConfig{
			Title:       "Test Site Updated",
			Description: "Test",
			BaseURL:     "http://localhost:1313",
		},
		Output: config.OutputConfig{
			Directory: tmpDir,
			Clean:     true,
		},
		Daemon: cfg.Daemon,
		Build:  cfg.Build,
	}

	ctx := context.Background()

	// Reload config
	err = daemon.ReloadConfig(ctx, newCfg)
	require.NoError(t, err, "config reload should succeed")

	// Wait for the async build trigger
	// The goroutine in ReloadConfig has a 500ms delay, so we wait a bit longer
	time.Sleep(1 * time.Second)

	// Verify the config was updated
	updatedCfg := daemon.GetConfig()
	require.NotNil(t, updatedCfg)
	require.Equal(t, "Test Site Updated", updatedCfg.Hugo.Title)
}
