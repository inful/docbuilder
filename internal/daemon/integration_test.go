package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestRefactoredDaemonIntegration tests the complete refactored daemon lifecycle
func TestRefactoredDaemonIntegration(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "daemon-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a minimal config for testing
	cfg := &config.Config{
		Version: "2.0",
		Output: config.OutputConfig{
			Directory: filepath.Join(tempDir, "output"),
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				AdminPort: 18080, // Use a different port to avoid conflicts
			},
			Sync: config.SyncConfig{
				QueueSize:        10,
				ConcurrentBuilds: 2,
			},
			Storage: config.StorageConfig{
				StateFile: filepath.Join(tempDir, "state.json"),
			},
		},
		Monitoring: &config.MonitoringConfig{
			Metrics: config.MonitoringMetrics{
				Enabled: true,
				Path:    "/metrics",
			},
			Health: config.MonitoringHealth{
				Path: "/health",
			},
		},
		Hugo: config.HugoConfig{
			BaseURL: "https://test.example.com",
		},
	}

	// Create the refactored daemon
	result := NewRefactoredDaemon(cfg)
	if result.IsErr() {
		t.Fatalf("Failed to create daemon: %v", result.UnwrapErr())
	}
	daemon := result.Unwrap()

	// Test that we can check status before starting
	if status := daemon.GetStatus(); status != StatusStopped {
		t.Errorf("Expected status %v, got %v", StatusStopped, status)
	}

	// Test service info after construction - services should already be registered
	services := daemon.GetServiceInfo()
	expectedServiceNames := []string{"state", "build-queue", "scheduler", "http-server", "config-watcher"}
	if len(services) != len(expectedServiceNames) {
		t.Errorf("Expected %d services after construction, got %d: %v", len(expectedServiceNames), len(services), services)
	}

	// Start the daemon
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := daemon.Start(ctx); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	// Check that the daemon started successfully
	if status := daemon.GetStatus(); status != StatusRunning {
		t.Errorf("Expected status %v, got %v", StatusRunning, status)
	}

	// Check that services were registered
	services = daemon.GetServiceInfo()
	if len(services) != len(expectedServiceNames) {
		t.Errorf("Expected %d services, got %d: %v", len(expectedServiceNames), len(services), services)
	}

	// Verify specific services are present
	serviceNames := make(map[string]bool)
	for _, service := range services {
		serviceNames[service.Name] = true
	}

	for _, expected := range expectedServiceNames {
		if !serviceNames[expected] {
			t.Errorf("Expected service %s not found in: %v", expected, services)
		}
	}

	// Stop the daemon
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := daemon.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop daemon: %v", err)
	}

	// Check that the daemon stopped
	if status := daemon.GetStatus(); status != StatusStopped {
		t.Errorf("Expected status %v, got %v", StatusStopped, status)
	}
}

// TestServiceOrchestrationPattern verifies the service orchestrator pattern is working
func TestServiceOrchestrationPattern(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "daemon-orchestration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		Version: "2.0",
		Output: config.OutputConfig{
			Directory: filepath.Join(tempDir, "output"),
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				AdminPort: 18081, // Different port
			},
			Sync: config.SyncConfig{
				QueueSize:        10,
				ConcurrentBuilds: 2,
			},
			Storage: config.StorageConfig{
				StateFile: filepath.Join(tempDir, "state.json"),
			},
		},
		Monitoring: &config.MonitoringConfig{
			Metrics: config.MonitoringMetrics{
				Enabled: true,
				Path:    "/metrics",
			},
			Health: config.MonitoringHealth{
				Path: "/health",
			},
		},
		Hugo: config.HugoConfig{
			BaseURL: "https://test.example.com",
		},
	}

	result := NewRefactoredDaemon(cfg)
	if result.IsErr() {
		t.Fatalf("Failed to create daemon: %v", result.UnwrapErr())
	}
	daemon := result.Unwrap()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := daemon.Start(ctx); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	// Test service health checks (via service info)
	services := daemon.GetServiceInfo()
	for _, service := range services {
		if service.Status == "" {
			t.Errorf("Service %s should have a status", service.Name)
		}
	}

	// Clean shutdown
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := daemon.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop daemon: %v", err)
	}
}
