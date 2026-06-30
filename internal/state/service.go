package state

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// Service is a service adapter that wraps the new state management system
// and integrates it with the service orchestrator. This bridges the gap between
// the monolithic StateManager and the new composable state stores.
type Service struct {
	store *JSONStore

	// Cached lifecycle values (formerly on ServiceAdapter).
	mu        sync.RWMutex
	loaded    bool
	lastSaved *time.Time
}

// NewService creates a new state service with the default JSON store.
func NewService(dataDir string) foundation.Result[*Service, error] {
	store := NewJSONStore(dataDir)
	if store.IsErr() {
		return foundation.Err[*Service, error](store.UnwrapErr())
	}

	return foundation.Ok[*Service, error](&Service{
		store: store.Unwrap(),
	})
}

// NewServiceWithStore creates a new state service with a custom store.
// This allows for dependency injection and testing with mock stores.
func NewServiceWithStore(store *JSONStore, dataDir string) *Service {
	return &Service{
		store: store,
	}
}

// Name returns the service name for the orchestrator.
func (ss *Service) Name() string {
	return "state"
}

// Start implements the services.ManagedService interface.
// This marks the state service as running and updates daemon status.
func (ss *Service) Start(ctx context.Context) error {
	// Test that the store is healthy
	health := ss.store.Health(ctx)
	if health.IsErr() {
		return errors.InternalError("state store health check failed").
			WithCause(health.UnwrapErr()).
			Build()
	}

	if health.Unwrap().Status != healthyStatus {
		return errors.InternalError("state store is unhealthy").
			WithContext("status", health.Unwrap().Status).
			WithContext("message", health.Unwrap().Message).
			Build()
	}

	// Update daemon status to running
	updateResult := ss.store.DaemonInfoUpdateStatus(ctx, "running")
	if updateResult.IsErr() {
		return errors.InternalError("failed to update daemon status to running").
			WithCause(updateResult.UnwrapErr()).
			Build()
	}

	return nil
}

// Stop implements the services.ManagedService interface.
// This gracefully shuts down the state service and ensures data is persisted.
func (ss *Service) Stop(ctx context.Context) error {
	// Update daemon status to stopping
	updateResult := ss.store.DaemonInfoUpdateStatus(ctx, "stopping")
	if updateResult.IsErr() {
		// Log error but continue with shutdown
		slog.Warn("failed to update daemon status during shutdown", "error", updateResult.UnwrapErr())
	}

	// Close the store to ensure data is persisted
	closeResult := ss.store.Close(ctx)
	if closeResult.IsErr() {
		return errors.InternalError("failed to close state store").
			WithCause(closeResult.UnwrapErr()).
			Build()
	}

	return nil
}

// Health implements the services.ManagedService interface.
func (ss *Service) Health() services.HealthStatus {
	ctx := context.Background()
	health := ss.store.Health(ctx)

	if health.IsErr() {
		return services.HealthStatus{
			Status:  "unhealthy",
			Message: fmt.Sprintf("store health check failed: %v", health.UnwrapErr()),
			CheckAt: health.Unwrap().CheckedAt,
		}
	}

	storeHealth := health.Unwrap()
	return services.HealthStatus{
		Status:  storeHealth.Status,
		Message: storeHealth.Message,
		CheckAt: storeHealth.CheckedAt,
	}
}

// Dependencies implements the services.ManagedService interface.
func (ss *Service) Dependencies() []string {
	return []string{} // State service has no dependencies
}

// --- LifecycleManager surface (formerly on ServiceAdapter) ---

// Load loads state from the underlying store. The typed JSON store
// loads on creation, so this is mostly a health check.
func (ss *Service) Load() error {
	ctx := context.Background()
	health := ss.store.Health(ctx)
	if health.IsErr() {
		return health.UnwrapErr()
	}
	if health.Unwrap().Status != healthyStatus {
		return errors.InternalError("state store unhealthy").
			WithContext("status", health.Unwrap().Status).
			Build()
	}
	ss.mu.Lock()
	ss.loaded = true
	ss.mu.Unlock()
	return nil
}

// Save persists state to the underlying store. The typed JSON store
// auto-persists on every mutation; this method just updates the cached
// lastSaved timestamp and triggers a health-check flush.
func (ss *Service) Save() error {
	ctx := context.Background()
	ss.mu.Lock()
	now := time.Now()
	ss.lastSaved = &now
	ss.mu.Unlock()

	health := ss.store.Health(ctx)
	if health.IsErr() {
		return health.UnwrapErr()
	}
	return nil
}

// IsLoaded returns whether the state has been loaded.
func (ss *Service) IsLoaded() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.loaded
}

// LastSaved returns the last save timestamp.
func (ss *Service) LastSaved() *time.Time {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.lastSaved
}

// Store returns the underlying *JSONStore for direct access.
// Callers can invoke the inlined per-capability methods directly
// (RepositoryCreate, ConfigurationGet, DaemonInfoUpdateStatus, etc).
func (ss *Service) Store() *JSONStore {
	return ss.store
}

// WithTransaction executes operations within a transaction-like context.
// This ensures consistency across multiple state operations.
func (ss *Service) WithTransaction(ctx context.Context, fn func(*JSONStore) error) foundation.Result[struct{}, error] {
	return ss.store.WithTransaction(ctx, fn)
}

// Migrate performs any necessary data migrations for schema changes.
// This would be called during service initialization if schema versions differ.
func (ss *Service) Migrate(_ context.Context, _, _ string) foundation.Result[struct{}, error] {
	// Placeholder for future migration logic
	// In a real implementation, this would handle schema changes between versions
	return foundation.Ok[struct{}, error](struct{}{})
}
