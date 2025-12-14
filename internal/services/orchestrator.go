// Package services provides service lifecycle management and dependency injection.
package services

import (
"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// ServiceStatus represents the current state of a managed service in the orchestrator.
type ServiceStatus string

const (
	// StatusNotStarted indicates the service has not yet started.
	StatusNotStarted ServiceStatus = "not_started"
	// StatusStarting indicates the service is in the process of starting.
	StatusStarting ServiceStatus = "starting"
	// StatusRunning indicates the service is running.
	StatusRunning ServiceStatus = "running"
	// StatusStopping indicates the service is in the process of stopping.
	StatusStopping ServiceStatus = "stopping"
	// StatusStopped indicates the service has stopped.
	StatusStopped ServiceStatus = "stopped"
	// StatusFailed indicates the service failed to start or run.
	StatusFailed ServiceStatus = "failed"
)

// HealthStatus represents the health of a managed service, including status, message, and timestamp.
type HealthStatus struct {
	Status  string    `json:"status"`
	Message string    `json:"message,omitempty"`
	CheckAt time.Time `json:"check_at"`
}

var (
	// HealthStatusHealthy is a reusable healthy status value.
	HealthStatusHealthy = HealthStatus{Status: "healthy", CheckAt: time.Now()}
	// HealthStatusUnhealthy returns a HealthStatus indicating an unhealthy state with a message.
	HealthStatusUnhealthy = func(message string) HealthStatus {
		return HealthStatus{Status: "unhealthy", Message: message, CheckAt: time.Now()}
	}
)

// ManagedService defines the interface for services managed by the ServiceOrchestrator.
type ManagedService interface {
	// Name returns the service name for logging and identification.
	Name() string

	// Start initializes and starts the service.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service.
	Stop(ctx context.Context) error

	// Health returns the current health status of the service.
	Health() HealthStatus

	// Dependencies returns the names of services this service depends on.
	Dependencies() []string
}

// ServiceInfo contains metadata and runtime status about a managed service, for reporting and diagnostics.
type ServiceInfo struct {
	Name         string        `json:"name"`
	Status       ServiceStatus `json:"status"`
	Health       HealthStatus  `json:"health"`
	Dependencies []string      `json:"dependencies"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	StoppedAt    *time.Time    `json:"stopped_at,omitempty"`
	LastError    string        `json:"last_error,omitempty"`
}

// ServiceOrchestrator manages the lifecycle of multiple services, including dependency resolution, start/stop sequencing, and health reporting.
type ServiceOrchestrator struct {
	services   map[string]ManagedService
	status     map[string]ServiceStatus
	startedAt  map[string]time.Time
	stoppedAt  map[string]time.Time
	lastErrors map[string]error
	mu         sync.RWMutex

	startTimeout time.Duration
	stopTimeout  time.Duration
}

// NewServiceOrchestrator creates a new ServiceOrchestrator with default timeouts.
func NewServiceOrchestrator() *ServiceOrchestrator {
	return &ServiceOrchestrator{
		services:     make(map[string]ManagedService),
		status:       make(map[string]ServiceStatus),
		startedAt:    make(map[string]time.Time),
		stoppedAt:    make(map[string]time.Time),
		lastErrors:   make(map[string]error),
		startTimeout: 30 * time.Second,
		stopTimeout:  10 * time.Second,
	}
}

// WithTimeouts sets custom start and stop timeouts for the orchestrator and returns itself for chaining.
func (so *ServiceOrchestrator) WithTimeouts(start, stop time.Duration) *ServiceOrchestrator {
	so.startTimeout = start
	so.stopTimeout = stop
	return so
}

// RegisterService adds a ManagedService to the orchestrator. Returns an error if the name is empty or already registered.
func (so *ServiceOrchestrator) RegisterService(service ManagedService) foundation.Result[struct{}, error] {
	so.mu.Lock()
	defer so.mu.Unlock()

	name := service.Name()
	if name == "" {
		return foundation.Err[struct{}, error](
			errors.ValidationError("service name cannot be empty").Build(),
		)
	}

	if _, exists := so.services[name]; exists {
		return foundation.Err[struct{}, error](
			errors.ValidationError(fmt.Sprintf("service %s already registered", name)).Build(),
		)
	}

	so.services[name] = service
	so.status[name] = StatusNotStarted

	slog.Debug("Service registered", "service", name, "dependencies", service.Dependencies())
	return foundation.Ok[struct{}, error](struct{}{})
}

// StartAll starts all registered services in dependency order, respecting declared dependencies. If any service fails to start, all started services are stopped.
func (so *ServiceOrchestrator) StartAll(ctx context.Context) error {
	so.mu.Lock()
	defer so.mu.Unlock()

	// Calculate start order based on dependencies
	startOrder, err := so.calculateStartOrder()
	if err != nil {
		return errors.InternalError("failed to calculate service start order").
			WithCause(err).
			Build()
	}

	slog.Info("Starting services", "count", len(startOrder), "order", startOrder)

	// Start services in order
	for _, serviceName := range startOrder {
		if err := so.startService(ctx, serviceName); err != nil {
			// Stop all started services on failure
			so.stopStartedServices(ctx)
			return err
		}
	}

	slog.Info("All services started successfully")
	return nil
}

// StopAll stops all registered services in reverse dependency order, ensuring dependents are stopped before their dependencies.
func (so *ServiceOrchestrator) StopAll(ctx context.Context) error {
	so.mu.Lock()
	defer so.mu.Unlock()

	// Calculate stop order (reverse of start order)
	startOrder, err := so.calculateStartOrder()
	if err != nil {
		return errors.InternalError("failed to calculate service stop order").
			WithCause(err).
			Build()
	}

	// Reverse the order for stopping
	stopOrder := make([]string, len(startOrder))
	for i, name := range startOrder {
		stopOrder[len(startOrder)-1-i] = name
	}

	slog.Info("Stopping services", "count", len(stopOrder), "order", stopOrder)

	var lastError error
	for _, serviceName := range stopOrder {
		if err := so.stopService(ctx, serviceName); err != nil {
			lastError = err
			slog.Error("Error stopping service", "service", serviceName, "error", err)
		}
	}

	if lastError != nil {
		return errors.InternalError("some services failed to stop gracefully").
			WithCause(lastError).
			Build()
	}

	slog.Info("All services stopped successfully")
	return nil
}

// GetServiceInfo returns ServiceInfo for a specific service name, or None if not found.
func (so *ServiceOrchestrator) GetServiceInfo(name string) foundation.Option[ServiceInfo] {
	so.mu.RLock()
	defer so.mu.RUnlock()

	service, exists := so.services[name]
	if !exists {
		return foundation.None[ServiceInfo]()
	}

	info := ServiceInfo{
		Name:         name,
		Status:       so.status[name],
		Dependencies: service.Dependencies(),
		Health:       service.Health(),
	}

	if startTime, exists := so.startedAt[name]; exists {
		info.StartedAt = &startTime
	}

	if stopTime, exists := so.stoppedAt[name]; exists {
		info.StoppedAt = &stopTime
	}

	if err, exists := so.lastErrors[name]; exists && err != nil {
		info.LastError = err.Error()
	}

	return foundation.Some(info)
}

// GetAllServiceInfo returns ServiceInfo for all registered services.
func (so *ServiceOrchestrator) GetAllServiceInfo() []ServiceInfo {
	so.mu.RLock()
	defer so.mu.RUnlock()

	var infos []ServiceInfo
	for name := range so.services {
		if info := so.GetServiceInfo(name); info.IsSome() {
			infos = append(infos, info.Unwrap())
		}
	}

	return infos
}

// calculateStartOrder determines the order in which services should be started using topological sort for dependency resolution.
func (so *ServiceOrchestrator) calculateStartOrder() ([]string, error) {
	// Topological sort to handle dependencies
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected involving service: %s", name)
		}

		if visited[name] {
			return nil
		}

		visiting[name] = true

		service, exists := so.services[name]
		if !exists {
			return fmt.Errorf("service not found: %s", name)
		}

		// Visit dependencies first
		for _, dep := range service.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		order = append(order, name)

		return nil
	}

	// Visit all services
	for name := range so.services {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// startService starts a single service by name, applying the configured start timeout.
func (so *ServiceOrchestrator) startService(ctx context.Context, name string) error {
	service := so.services[name]
	so.status[name] = StatusStarting

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, so.startTimeout)
	defer cancel()

	slog.Debug("Starting service", "service", name)
	startTime := time.Now()

	err := service.Start(timeoutCtx)
	if err != nil {
		so.status[name] = StatusFailed
		so.lastErrors[name] = err
		return errors.InternalError(fmt.Sprintf("failed to start service %s", name)).
			WithCause(err).
			Build()
	}

	so.status[name] = StatusRunning
	so.startedAt[name] = startTime
	so.lastErrors[name] = nil

	slog.Info("Service started", "service", name, "duration", time.Since(startTime))
	return nil
}

// stopService stops a single service by name, applying the configured stop timeout.
func (so *ServiceOrchestrator) stopService(ctx context.Context, name string) error {
	service := so.services[name]

	// Only stop if running
	if so.status[name] != StatusRunning {
		return nil
	}

	so.status[name] = StatusStopping

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, so.stopTimeout)
	defer cancel()

	slog.Debug("Stopping service", "service", name)
	stopTime := time.Now()

	err := service.Stop(timeoutCtx)
	if err != nil {
		so.status[name] = StatusFailed
		so.lastErrors[name] = err
		return err
	}

	so.status[name] = StatusStopped
	so.stoppedAt[name] = stopTime

	slog.Info("Service stopped", "service", name, "duration", time.Since(stopTime))
	return nil
}

// stopStartedServices stops all currently running services. Used for cleanup if a start failure occurs.
func (so *ServiceOrchestrator) stopStartedServices(ctx context.Context) {
	for name, status := range so.status {
		if status == StatusRunning {
			if err := so.stopService(ctx, name); err != nil {
				slog.Error("Error stopping service during cleanup", "service", name, "error", err)
			}
		}
	}
}
