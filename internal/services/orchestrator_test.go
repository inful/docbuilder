package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// MockService is a test implementation of ManagedService.
type MockService struct {
	name         string
	dependencies []string
	startDelay   time.Duration
	stopDelay    time.Duration
	failStart    bool
	failStop     bool
	isRunning    bool
	mu           sync.Mutex
}

func NewMockService(name string, deps ...string) *MockService {
	return &MockService{
		name:         name,
		dependencies: deps,
	}
}

func (m *MockService) WithStartDelay(delay time.Duration) *MockService {
	m.startDelay = delay
	return m
}

func (m *MockService) WithStopDelay(delay time.Duration) *MockService {
	m.stopDelay = delay
	return m
}

func (m *MockService) WithStartFailure() *MockService {
	m.failStart = true
	return m
}

func (m *MockService) WithStopFailure() *MockService {
	m.failStop = true
	return m
}

func (m *MockService) Name() string {
	return m.name
}

func (m *MockService) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.startDelay > 0 {
		select {
		case <-time.After(m.startDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.failStart {
		return errors.New("mock start failure")
	}

	m.isRunning = true
	return nil
}

func (m *MockService) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopDelay > 0 {
		select {
		case <-time.After(m.stopDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.failStop {
		return errors.New("mock stop failure")
	}

	m.isRunning = false
	return nil
}

func (m *MockService) Health() HealthStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy("service not running")
}

func (m *MockService) Dependencies() []string {
	return m.dependencies
}

func (m *MockService) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

func TestServiceOrchestrator(t *testing.T) {
	t.Run("Single service lifecycle", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator()
		service := NewMockService("test-service")

		// Register service
		result := orchestrator.RegisterService(service)
		if result.IsErr() {
			t.Fatalf("Failed to register service: %v", result.UnwrapErr())
		}

		// Start service
		ctx := context.Background()
		if err := orchestrator.StartAll(ctx); err != nil {
			t.Fatalf("Failed to start services: %v", err)
		}

		// Verify service is running
		if !service.IsRunning() {
			t.Error("Expected service to be running")
		}

		// Check service info
		info := orchestrator.GetServiceInfo("test-service")
		if info.IsNone() {
			t.Error("Expected to find service info")
		} else {
			serviceInfo := info.Unwrap()
			if serviceInfo.Status != StatusRunning {
				t.Errorf("Expected status running, got %s", serviceInfo.Status)
			}
			if serviceInfo.Health.Status != "healthy" {
				t.Errorf("Expected healthy status, got %s", serviceInfo.Health.Status)
			}
		}

		// Stop service
		if err := orchestrator.StopAll(ctx); err != nil {
			t.Fatalf("Failed to stop services: %v", err)
		}

		// Verify service is stopped
		if service.IsRunning() {
			t.Error("Expected service to be stopped")
		}
	})

	t.Run("Dependency resolution", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator()

		// Create services with dependencies
		serviceA := NewMockService("service-a")
		serviceB := NewMockService("service-b", "service-a")
		serviceC := NewMockService("service-c", "service-a", "service-b")

		// Register services (out of order to test dependency resolution)
		orchestrator.RegisterService(serviceC)
		orchestrator.RegisterService(serviceA)
		orchestrator.RegisterService(serviceB)

		// Start all services
		ctx := context.Background()
		if err := orchestrator.StartAll(ctx); err != nil {
			t.Fatalf("Failed to start services: %v", err)
		}

		// Verify all services are running
		for _, service := range []*MockService{serviceA, serviceB, serviceC} {
			if !service.IsRunning() {
				t.Errorf("Expected service %s to be running", service.Name())
			}
		}

		// Stop all services
		if err := orchestrator.StopAll(ctx); err != nil {
			t.Fatalf("Failed to stop services: %v", err)
		}
	})

	t.Run("Circular dependency detection", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator()

		// Create services with circular dependencies
		serviceA := NewMockService("service-a", "service-b")
		serviceB := NewMockService("service-b", "service-a")

		orchestrator.RegisterService(serviceA)
		orchestrator.RegisterService(serviceB)

		// Start should fail due to circular dependency
		ctx := context.Background()
		err := orchestrator.StartAll(ctx)
		if err == nil {
			t.Error("Expected error due to circular dependency")
		}

		var classified *foundation.ClassifiedError
		if foundation.AsClassified(err, &classified) {
			if classified.Code != foundation.ErrorCodeInternal {
				t.Error("Expected internal error code for dependency cycle")
			}
		}
	})

	t.Run("Service start failure", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator()

		serviceA := NewMockService("service-a")
		serviceB := NewMockService("service-b").WithStartFailure()
		serviceC := NewMockService("service-c", "service-b")

		orchestrator.RegisterService(serviceA)
		orchestrator.RegisterService(serviceB)
		orchestrator.RegisterService(serviceC)

		// Start should fail when service-b fails
		ctx := context.Background()
		err := orchestrator.StartAll(ctx)
		if err == nil {
			t.Error("Expected error due to service start failure")
		}

		// Verify service-a is stopped (cleanup)
		if serviceA.IsRunning() {
			t.Error("Expected service-a to be stopped after failure cleanup")
		}

		// Verify service-c never started
		if serviceC.IsRunning() {
			t.Error("Expected service-c to never start due to dependency failure")
		}
	})

	t.Run("Timeout handling", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator().WithTimeouts(50*time.Millisecond, 50*time.Millisecond)

		// Service that takes too long to start
		slowService := NewMockService("slow-service").WithStartDelay(100 * time.Millisecond)

		orchestrator.RegisterService(slowService)

		// Start should timeout
		ctx := context.Background()
		err := orchestrator.StartAll(ctx)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("Service registration validation", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator()

		// Test empty name validation
		emptyNameService := &MockService{name: ""}
		result := orchestrator.RegisterService(emptyNameService)
		if result.IsOk() {
			t.Error("Expected error for empty service name")
		}

		// Test duplicate registration
		service := NewMockService("test")
		orchestrator.RegisterService(service)

		duplicateResult := orchestrator.RegisterService(service)
		if duplicateResult.IsOk() {
			t.Error("Expected error for duplicate service registration")
		}
	})

	t.Run("Service info retrieval", func(t *testing.T) {
		orchestrator := NewServiceOrchestrator()
		service := NewMockService("info-test", "dependency")

		orchestrator.RegisterService(service)

		// Get info for existing service
		info := orchestrator.GetServiceInfo("info-test")
		if info.IsNone() {
			t.Error("Expected to find service info")
		} else {
			serviceInfo := info.Unwrap()
			if serviceInfo.Name != "info-test" {
				t.Error("Expected correct service name")
			}
			if len(serviceInfo.Dependencies) != 1 || serviceInfo.Dependencies[0] != "dependency" {
				t.Error("Expected correct dependencies")
			}
		}

		// Get info for non-existent service
		noInfo := orchestrator.GetServiceInfo("non-existent")
		if noInfo.IsSome() {
			t.Error("Expected no info for non-existent service")
		}

		// Get all service info
		allInfo := orchestrator.GetAllServiceInfo()
		if len(allInfo) != 1 {
			t.Errorf("Expected 1 service info, got %d", len(allInfo))
		}
	})
}
