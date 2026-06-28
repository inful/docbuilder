package state

import (
	"context"
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// Service is a service adapter that wraps the new state management system
// and integrates it with the service orchestrator. This bridges the gap between
// the monolithic StateManager and the new composable state stores.
type Service struct {
	store Store
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
func NewServiceWithStore(store Store, dataDir string) *Service {
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
	daemonStore := ss.store.DaemonInfo()
	updateResult := daemonStore.UpdateStatus(ctx, "running")
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
	daemonStore := ss.store.DaemonInfo()
	updateResult := daemonStore.UpdateStatus(ctx, "stopping")
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

// Store returns the underlying state store for direct access.
// This allows other services to interact with state through the interfaces.
func (ss *Service) Store() Store {
	return ss.store
}

// GetRepositoryStore provides typed access to repository operations.
func (ss *Service) GetRepositoryStore() RepositoryStore {
	return ss.store.Repositories()
}

// GetBuildStore provides typed access to build operations.
func (ss *Service) GetBuildStore() BuildStore {
	return ss.store.Builds()
}

// GetScheduleStore provides typed access to schedule operations.
func (ss *Service) GetScheduleStore() ScheduleStore {
	return ss.store.Schedules()
}

// GetStatisticsStore provides typed access to statistics operations.
func (ss *Service) GetStatisticsStore() StatisticsStore {
	return ss.store.Statistics()
}

// GetConfigurationStore provides typed access to configuration operations.
func (ss *Service) GetConfigurationStore() ConfigurationStore {
	return ss.store.Configuration()
}

// GetDaemonInfoStore provides typed access to daemon info operations.
func (ss *Service) GetDaemonInfoStore() DaemonInfoStore {
	return ss.store.DaemonInfo()
}

// WithTransaction executes operations within a transaction-like context.
// This ensures consistency across multiple state operations.
func (ss *Service) WithTransaction(ctx context.Context, fn func(Store) error) foundation.Result[struct{}, error] {
	return ss.store.WithTransaction(ctx, fn)
}

// Migrate performs any necessary data migrations for schema changes.
// This would be called during service initialization if schema versions differ.
func (ss *Service) Migrate(_ context.Context, _, _ string) foundation.Result[struct{}, error] {
	// Placeholder for future migration logic
	// In a real implementation, this would handle schema changes between versions
	return foundation.Ok[struct{}, error](struct{}{})
}

// Compact performs maintenance operations on the state store.
// For the JSON store, this might involve cleaning up old builds, compacting data, etc.
func (ss *Service) Compact(ctx context.Context) foundation.Result[struct{}, error] {
	// Clean up old builds to prevent unbounded growth
	buildStore := ss.GetBuildStore()
	cleanupResult := buildStore.Cleanup(ctx, 1000) // Keep last 1000 builds
	if cleanupResult.IsErr() {
		return foundation.Err[struct{}, error](
			errors.InternalError("failed to cleanup old builds").
				WithCause(cleanupResult.UnwrapErr()).
				Build(),
		)
	}

	// Could add other maintenance operations here:
	// - Statistics cleanup
	// - Configuration validation
	// - Data integrity checks

	return foundation.Ok[struct{}, error](struct{}{})
}
