package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher monitors configuration file changes and triggers reloads
type ConfigWatcher struct {
	configPath   string
	daemon       *Daemon
	watcher      *fsnotify.Watcher
	mu           sync.RWMutex
	stopChan     chan struct{}
	reloadChan   chan struct{}
	debounceTime time.Duration
}

// NewConfigWatcher creates a new configuration file watcher
func NewConfigWatcher(configPath string, daemon *Daemon) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Resolve absolute path for consistent watching
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	return &ConfigWatcher{
		configPath:   absPath,
		daemon:       daemon,
		watcher:      watcher,
		stopChan:     make(chan struct{}),
		reloadChan:   make(chan struct{}, 1),
		debounceTime: 2 * time.Second, // Debounce rapid file changes
	}, nil
}

// Start begins monitoring the configuration file
func (cw *ConfigWatcher) Start(ctx context.Context) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Watch the directory containing the config file (more reliable than watching the file directly)
	configDir := filepath.Dir(cw.configPath)
	if err := cw.watcher.Add(configDir); err != nil {
		return fmt.Errorf("failed to watch config directory %s: %w", configDir, err)
	}

	slog.Info("Starting configuration watcher", "config_path", cw.configPath)

	// Start the watcher goroutines
	go cw.watchLoop(ctx)
	go cw.reloadLoop(ctx)

	return nil
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop(ctx context.Context) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	slog.Info("Stopping configuration watcher")

	// Signal stop to all goroutines
	close(cw.stopChan)

	// Close the file system watcher
	if cw.watcher != nil {
		if err := cw.watcher.Close(); err != nil {
			slog.Error("Error closing file watcher", "error", err)
		}
	}

	return nil
}

// watchLoop monitors file system events
func (cw *ConfigWatcher) watchLoop(ctx context.Context) {
	configFile := filepath.Base(cw.configPath)

	for {
		select {
		case <-ctx.Done():
			return
		case <-cw.stopChan:
			return
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Only process events for our config file
			if filepath.Base(event.Name) != configFile {
				continue
			}

			// Handle different types of file events
			if event.Op&fsnotify.Write == fsnotify.Write {
				slog.Debug("Config file write detected", "file", event.Name)
				cw.triggerReload()
			} else if event.Op&fsnotify.Create == fsnotify.Create {
				slog.Debug("Config file create detected", "file", event.Name)
				cw.triggerReload()
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				slog.Warn("Config file removed", "file", event.Name)
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				slog.Debug("Config file rename detected", "file", event.Name)
				cw.triggerReload()
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Config watcher error", "error", err)
		}
	}
}

// reloadLoop handles debounced configuration reloads
func (cw *ConfigWatcher) reloadLoop(ctx context.Context) {
	var reloadTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if reloadTimer != nil {
				reloadTimer.Stop()
			}
			return
		case <-cw.stopChan:
			if reloadTimer != nil {
				reloadTimer.Stop()
			}
			return
		case <-cw.reloadChan:
			// Reset/start debounce timer
			if reloadTimer != nil {
				reloadTimer.Stop()
			}
			reloadTimer = time.AfterFunc(cw.debounceTime, func() {
				if err := cw.performReload(ctx); err != nil {
					slog.Error("Failed to reload configuration", "error", err)
				}
			})
		}
	}
}

// triggerReload triggers a debounced configuration reload
func (cw *ConfigWatcher) triggerReload() {
	select {
	case cw.reloadChan <- struct{}{}:
		// Reload triggered
	default:
		// Reload already pending
	}
}

// performReload loads and applies the new configuration
func (cw *ConfigWatcher) performReload(ctx context.Context) error {
	slog.Info("Reloading configuration", "config_path", cw.configPath)

	// Load the new configuration
	newConfig, err := config.LoadV2(cw.configPath)
	if err != nil {
		return fmt.Errorf("failed to load new configuration: %w", err)
	}

	// Validate the new configuration
	if err := cw.validateConfigChange(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply the new configuration to the daemon
	if err := cw.daemon.ReloadConfig(ctx, newConfig); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	slog.Info("Configuration reloaded successfully")
	return nil
}

// validateConfigChange validates that the configuration change is safe to apply
func (cw *ConfigWatcher) validateConfigChange(newConfig *config.V2Config) error {
	currentConfig := cw.daemon.GetConfig()

	// Check for critical changes that might require daemon restart
	if newConfig.Version != currentConfig.Version {
		return fmt.Errorf("configuration version change requires daemon restart")
	}

	// Validate HTTP port changes
	if newConfig.Daemon != nil && currentConfig.Daemon != nil {
		if newConfig.Daemon.HTTP.DocsPort != currentConfig.Daemon.HTTP.DocsPort ||
			newConfig.Daemon.HTTP.AdminPort != currentConfig.Daemon.HTTP.AdminPort ||
			newConfig.Daemon.HTTP.WebhookPort != currentConfig.Daemon.HTTP.WebhookPort {
			slog.Warn("HTTP port changes detected - may require restart for full effect")
		}
	}

	// Additional validation logic can be added here
	// - Check for forge authentication changes
	// - Validate new repository configurations
	// - Ensure versioning strategies are compatible

	return nil
}
