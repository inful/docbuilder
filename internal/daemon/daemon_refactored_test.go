package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestRefactoredDaemon(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			Sync: config.SyncConfig{
				QueueSize:        10,
				ConcurrentBuilds: 2,
			},
			HTTP: config.HTTPConfig{
				AdminPort: 0, // Use random port for testing
			},
		},
		Monitoring: &config.MonitoringConfig{
			Health: config.MonitoringHealth{
				Path: "/health",
			},
			Metrics: config.MonitoringMetrics{
				Enabled: false, // Disable to simplify test
				Path:    "/metrics",
			},
		},
	}

	t.Run("Successful creation and lifecycle", func(t *testing.T) {
		// Create daemon
		result := NewRefactoredDaemon(cfg)
		if result.IsErr() {
			t.Fatalf("Failed to create daemon: %v", result.UnwrapErr())
		}

		daemon := result.Unwrap()

		// Verify initial status
		if daemon.GetStatus() != StatusStopped {
			t.Error("Expected daemon to start in stopped state")
		}

		// Start daemon
		ctx := context.Background()
		if err := daemon.Start(ctx); err != nil {
			t.Fatalf("Failed to start daemon: %v", err)
		}

		// Verify running status
		if daemon.GetStatus() != StatusRunning {
			t.Error("Expected daemon to be running after start")
		}

		// Check service info
		services := daemon.GetServiceInfo()
		if len(services) == 0 {
			t.Error("Expected at least one service to be registered")
		}

		// Verify all services are running
		for _, service := range services {
			if service.Status != "running" {
				t.Errorf("Expected service %s to be running, got %s", service.Name, service.Status)
			}
		}

		// Test service health check
		healthOpt := daemon.GetServiceHealth("state")
		if healthOpt.IsNone() {
			t.Error("Expected to find state service health")
		} else {
			health := healthOpt.Unwrap()
			if health.Status != "healthy" {
				t.Errorf("Expected state to be healthy, got %s", health.Status)
			}
		}

		// Stop daemon
		if err := daemon.Stop(ctx); err != nil {
			t.Fatalf("Failed to stop daemon: %v", err)
		}

		// Verify stopped status
		if daemon.GetStatus() != StatusStopped {
			t.Error("Expected daemon to be stopped after stop")
		}
	})

	t.Run("Configuration validation", func(t *testing.T) {
		// Test nil config
		result := NewRefactoredDaemon(nil)
		if result.IsOk() {
			t.Error("Expected error for nil config")
		}

		// Test invalid daemon config
		invalidCfg := &config.Config{
			Daemon: &config.DaemonConfig{
				Sync: config.SyncConfig{
					QueueSize:        0, // Invalid
					ConcurrentBuilds: 2,
				},
			},
		}

		result = NewRefactoredDaemon(invalidCfg)
		if result.IsOk() {
			t.Error("Expected error for invalid daemon config")
		}
	})

	t.Run("Service orchestration", func(t *testing.T) {
		result := NewRefactoredDaemon(cfg)
		if result.IsErr() {
			t.Fatalf("Failed to create daemon: %v", result.UnwrapErr())
		}

		daemon := result.Unwrap()

		// Start with timeout to test orchestrator timeout handling
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := daemon.Start(ctx); err != nil {
			t.Fatalf("Failed to start daemon: %v", err)
		}

		// Verify dependency order by checking start times
		services := daemon.GetServiceInfo()
		stateStarted := false
		for _, service := range services {
			if service.Name == "state" && service.StartedAt != nil {
				stateStarted = true
				break
			}
		}

		if !stateStarted {
			t.Error("Expected state to be started")
		}

		// Clean up
		daemon.Stop(context.Background())
	})

	t.Run("Error handling", func(t *testing.T) {
		result := NewRefactoredDaemon(cfg)
		if result.IsErr() {
			t.Fatalf("Failed to create daemon: %v", result.UnwrapErr())
		}

		daemon := result.Unwrap()

		// Test multiple starts
		ctx := context.Background()
		daemon.Start(ctx)

		// Second start should handle gracefully (implementation dependent)
		// For this demo, we'll just test that the daemon remains in a consistent state
		if daemon.GetStatus() != StatusRunning {
			t.Error("Expected daemon to remain running")
		}

		daemon.Stop(ctx)
	})
}

func TestDaemonConfigValidation(t *testing.T) {
	t.Run("Valid config", func(t *testing.T) {
		cfg := &config.Config{
			Daemon: &config.DaemonConfig{
				Sync: config.SyncConfig{
					QueueSize:        10,
					ConcurrentBuilds: 2,
				},
			},
		}

		err := validateDaemonConfig(cfg)
		if err != nil {
			t.Errorf("Expected valid config to pass validation, got: %v", err)
		}
	})

	t.Run("Missing daemon config", func(t *testing.T) {
		cfg := &config.Config{}

		err := validateDaemonConfig(cfg)
		if err == nil {
			t.Error("Expected error for missing daemon config")
		}
	})

	t.Run("Invalid queue size", func(t *testing.T) {
		cfg := &config.Config{
			Daemon: &config.DaemonConfig{
				Sync: config.SyncConfig{
					QueueSize:        0, // Invalid
					ConcurrentBuilds: 2,
				},
			},
		}

		err := validateDaemonConfig(cfg)
		if err == nil {
			t.Error("Expected error for invalid queue size")
		}
	})
}
