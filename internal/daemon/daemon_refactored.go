// Package daemon demonstrates how to refactor the large daemon.go file using
// the service orchestrator pattern and foundation utilities.
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/services"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// RefactoredDaemon demonstrates the new streamlined daemon using service orchestration.
// This replaces the 683-line monolithic daemon.go with a focused orchestrator.
type RefactoredDaemon struct {
	config       *config.Config
	status       atomic.Value // DaemonStatus (from original)
	orchestrator *services.ServiceOrchestrator
	startTime    time.Time
}

// NewRefactoredDaemon creates a new daemon instance using the service orchestrator pattern.
func NewRefactoredDaemon(cfg *config.Config) foundation.Result[*RefactoredDaemon, error] {
	// Validate configuration using foundation utilities
	if err := validateDaemonConfig(cfg); err != nil {
		return foundation.Err[*RefactoredDaemon, error](err)
	}

	daemon := &RefactoredDaemon{
		config:       cfg,
		orchestrator: services.NewServiceOrchestrator().WithTimeouts(30*time.Second, 10*time.Second),
	}

	daemon.status.Store(StatusStopped)

	// Register all daemon services with the orchestrator
	if err := daemon.registerServices(); err != nil {
		return foundation.Err[*RefactoredDaemon, error](err)
	}

	return foundation.Ok[*RefactoredDaemon, error](daemon)
}

// validateDaemonConfig demonstrates foundation-based validation.
func validateDaemonConfig(cfg *config.Config) error {
	validator := foundation.NewValidatorChain(
		foundation.Required[*config.Config]("config"),
		foundation.Custom("config", "daemon_required", "daemon configuration is required",
			func(c *config.Config) bool {
				return c != nil && c.Daemon != nil
			}),
		foundation.Custom("config", "valid_daemon", "daemon configuration is invalid",
			func(c *config.Config) bool {
				if c == nil || c.Daemon == nil {
					return false
				}
				// Add more daemon-specific validation here
				return c.Daemon.Sync.QueueSize > 0 && c.Daemon.Sync.ConcurrentBuilds > 0
			}),
	)

	result := validator.Validate(cfg)
	return result.ToError()
}

// registerServices registers all daemon components as managed services.
func (d *RefactoredDaemon) registerServices() error {
	// Register the new state management service from Phase 2
	stateDataDir := "./daemon-data" // Default data directory
	if d.config.Daemon != nil && d.config.Daemon.Storage.StateFile != "" {
		stateDataDir = filepath.Dir(d.config.Daemon.Storage.StateFile)
	}

	// Create and register the state service
	stateServiceResult := state.NewService(stateDataDir)
	if stateServiceResult.IsErr() {
		return fmt.Errorf("failed to create state service: %w", stateServiceResult.UnwrapErr())
	}
	stateService := stateServiceResult.Unwrap()

	if result := d.orchestrator.RegisterService(stateService); result.IsErr() {
		return fmt.Errorf("failed to register state service: %w", result.UnwrapErr())
	}

	// Create a temporary daemon instance for service dependencies
	// Note: This is a simplification for the demo - in a real implementation,
	// we'd extract the interfaces needed by each service to avoid circular dependencies
	tempDaemon := &Daemon{
		config:    d.config,
		startTime: time.Now(),
	}
	// Initialize the status to prevent interface conversion panics
	tempDaemon.status.Store(StatusStopped)

	// Register build queue service
	buildQueueService := NewBuildQueueService("build-queue", d.config)
	if result := d.orchestrator.RegisterService(buildQueueService); result.IsErr() {
		return fmt.Errorf("failed to register build queue service: %w", result.UnwrapErr())
	}

	// Register scheduler service (depends on build queue)
	// Note: We'll need to get the build queue instance after it's started
	schedulerService := NewSchedulerService("scheduler", tempDaemon, buildQueueService.GetBuildQueue())
	if result := d.orchestrator.RegisterService(schedulerService); result.IsErr() {
		return fmt.Errorf("failed to register scheduler service: %w", result.UnwrapErr())
	}

	// Register HTTP server service
	httpServerService := NewHTTPServerService("http-server", tempDaemon, d.config)
	if result := d.orchestrator.RegisterService(httpServerService); result.IsErr() {
		return fmt.Errorf("failed to register HTTP server service: %w", result.UnwrapErr())
	}

	// Register config watcher service (if config file is available)
	configPath := "" // Would be passed from main if available
	configWatcherService := NewConfigWatcherService("config-watcher", tempDaemon, configPath)
	if result := d.orchestrator.RegisterService(configWatcherService); result.IsErr() {
		return fmt.Errorf("failed to register config watcher service: %w", result.UnwrapErr())
	}

	return nil
}

// Start starts the daemon using the service orchestrator.
// This replaces ~100 lines of manual service startup with orchestrated lifecycle management.
func (d *RefactoredDaemon) Start(ctx context.Context) error {
	d.status.Store(StatusStarting)
	d.startTime = time.Now()

	slog.Info("Starting RefactoredDaemon", "services", len(d.orchestrator.GetAllServiceInfo()))

	if err := d.orchestrator.StartAll(ctx); err != nil {
		d.status.Store(StatusError)
		return err
	}

	d.status.Store(StatusRunning)
	slog.Info("RefactoredDaemon started successfully", "duration", time.Since(d.startTime))

	return nil
}

// Stop stops the daemon using the service orchestrator.
// This replaces ~50 lines of manual service shutdown with orchestrated lifecycle management.
func (d *RefactoredDaemon) Stop(ctx context.Context) error {
	d.status.Store(StatusStopping)

	slog.Info("Stopping RefactoredDaemon")

	if err := d.orchestrator.StopAll(ctx); err != nil {
		d.status.Store(StatusError)
		return err
	}

	d.status.Store(StatusStopped)
	slog.Info("RefactoredDaemon stopped successfully")

	return nil
}

// GetStatus returns the current daemon status.
func (d *RefactoredDaemon) GetStatus() Status {
	return d.status.Load().(Status)
}

// GetServiceInfo returns information about all managed services.
func (d *RefactoredDaemon) GetServiceInfo() []services.ServiceInfo {
	return d.orchestrator.GetAllServiceInfo()
}

// GetServiceHealth returns the health status of a specific service.
func (d *RefactoredDaemon) GetServiceHealth(name string) foundation.Option[services.HealthStatus] {
	info := d.orchestrator.GetServiceInfo(name)
	return foundation.MapOption(info, func(si services.ServiceInfo) services.HealthStatus {
		return si.Health
	})
}
