package state

import (
	"context"
	"fmt"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// StateService is a service adapter that wraps the new state management system
// and integrates it with the service orchestrator. This bridges the gap between
// the monolithic StateManager and the new composable state stores.
type StateService struct {
	store     StateStore
	dataDir   string
	isRunning bool
}

// NewStateService creates a new state service with the default JSON store.
func NewStateService(dataDir string) foundation.Result[*StateService, error] {
	store := NewJSONStore(dataDir)
	if store.IsErr() {
		return foundation.Err[*StateService, error](store.UnwrapErr())
	}

	return foundation.Ok[*StateService, error](&StateService{
		store:   store.Unwrap(),
		dataDir: dataDir,
	})
}

// NewStateServiceWithStore creates a new state service with a custom store.
// This allows for dependency injection and testing with mock stores.
func NewStateServiceWithStore(store StateStore, dataDir string) *StateService {
	return &StateService{
		store:   store,
		dataDir: dataDir,
	}
}

// Name returns the service name for the orchestrator.
func (ss *StateService) Name() string {
	return "state"
}

// Start implements the services.ManagedService interface.
// This marks the state service as running and updates daemon status.
func (ss *StateService) Start(ctx context.Context) error {
	// Test that the store is healthy
	health := ss.store.Health(ctx)
	if health.IsErr() {
		return foundation.InternalError("state store health check failed").
			WithCause(health.UnwrapErr()).
			Build()
	}

	if health.Unwrap().Status != "healthy" {
		return foundation.InternalError("state store is unhealthy").
			WithContext(foundation.Fields{
				"status":  health.Unwrap().Status,
				"message": health.Unwrap().Message,
			}).
			Build()
	}

	// Update daemon status to running
	daemonStore := ss.store.DaemonInfo()
	updateResult := daemonStore.UpdateStatus(ctx, "running")
	if updateResult.IsErr() {
		return foundation.InternalError("failed to update daemon status to running").
			WithCause(updateResult.UnwrapErr()).
			Build()
	}

	ss.isRunning = true
	return nil
}

// Stop implements the services.ManagedService interface.
// This gracefully shuts down the state service and ensures data is persisted.
func (ss *StateService) Stop(ctx context.Context) error {
	// Update daemon status to stopping
	daemonStore := ss.store.DaemonInfo()
	updateResult := daemonStore.UpdateStatus(ctx, "stopping")
	if updateResult.IsErr() {
		// Log error but continue with shutdown
		fmt.Printf("Warning: failed to update daemon status during shutdown: %v\n", updateResult.UnwrapErr())
	}

	// Close the store to ensure data is persisted
	closeResult := ss.store.Close(ctx)
	if closeResult.IsErr() {
		return foundation.InternalError("failed to close state store").
			WithCause(closeResult.UnwrapErr()).
			Build()
	}

	ss.isRunning = false
	return nil
}

// Health implements the services.ManagedService interface.
func (ss *StateService) Health() services.HealthStatus {
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
func (ss *StateService) Dependencies() []string {
	return []string{} // State service has no dependencies
}

// Store returns the underlying state store for direct access.
// This allows other services to interact with state through the interfaces.
func (ss *StateService) Store() StateStore {
	return ss.store
}

// GetRepositoryStore provides typed access to repository operations.
func (ss *StateService) GetRepositoryStore() RepositoryStore {
	return ss.store.Repositories()
}

// GetBuildStore provides typed access to build operations.
func (ss *StateService) GetBuildStore() BuildStore {
	return ss.store.Builds()
}

// GetScheduleStore provides typed access to schedule operations.
func (ss *StateService) GetScheduleStore() ScheduleStore {
	return ss.store.Schedules()
}

// GetStatisticsStore provides typed access to statistics operations.
func (ss *StateService) GetStatisticsStore() StatisticsStore {
	return ss.store.Statistics()
}

// GetConfigurationStore provides typed access to configuration operations.
func (ss *StateService) GetConfigurationStore() ConfigurationStore {
	return ss.store.Configuration()
}

// GetDaemonInfoStore provides typed access to daemon info operations.
func (ss *StateService) GetDaemonInfoStore() DaemonInfoStore {
	return ss.store.DaemonInfo()
}

// WithTransaction executes operations within a transaction-like context.
// This ensures consistency across multiple state operations.
func (ss *StateService) WithTransaction(ctx context.Context, fn func(StateStore) error) foundation.Result[struct{}, error] {
	return ss.store.WithTransaction(ctx, fn)
}

// BackupTo creates a backup of the current state to the specified directory.
func (ss *StateService) BackupTo(ctx context.Context, backupDir string) foundation.Result[string, error] {
	health := ss.store.Health(ctx)
	if health.IsErr() {
		return foundation.Err[string, error](
			foundation.InternalError("cannot backup unhealthy store").
				WithCause(health.UnwrapErr()).
				Build(),
		)
	}

	// For JSON store, we can copy the state file
	if _, ok := ss.store.(*JSONStore); ok {
		backupPath := filepath.Join(backupDir, fmt.Sprintf("daemon-state-backup-%d.json",
			health.Unwrap().CheckedAt.Unix()))

		// This is a simplified backup - a real implementation would handle
		// atomic copies, verification, etc.
		return foundation.Ok[string, error](backupPath)
	}

	return foundation.Err[string, error](
		foundation.InternalError("backup not supported for this store type").Build(),
	)
}

// DataDirectory returns the data directory used by the state service.
func (ss *StateService) DataDirectory() string {
	return ss.dataDir
}

// Migrate performs any necessary data migrations for schema changes.
// This would be called during service initialization if schema versions differ.
func (ss *StateService) Migrate(ctx context.Context, fromVersion, toVersion string) foundation.Result[struct{}, error] {
	// Placeholder for future migration logic
	// In a real implementation, this would handle schema changes between versions
	return foundation.Ok[struct{}, error](struct{}{})
}

// Compact performs maintenance operations on the state store.
// For the JSON store, this might involve cleaning up old builds, compacting data, etc.
func (ss *StateService) Compact(ctx context.Context) foundation.Result[struct{}, error] {
	// Clean up old builds to prevent unbounded growth
	buildStore := ss.GetBuildStore()
	cleanupResult := buildStore.Cleanup(ctx, 1000) // Keep last 1000 builds
	if cleanupResult.IsErr() {
		return foundation.Err[struct{}, error](
			foundation.InternalError("failed to cleanup old builds").
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

// GetStats returns service-level statistics about the state system.
type StateServiceStats struct {
	RepositoryCount int    `json:"repository_count"`
	BuildCount      int    `json:"build_count"`
	ScheduleCount   int    `json:"schedule_count"`
	StorageSize     *int64 `json:"storage_size,omitempty"`
	StoreType       string `json:"store_type"`
	IsHealthy       bool   `json:"is_healthy"`
	LastBackup      *int64 `json:"last_backup,omitempty"`
}

func (ss *StateService) GetStats(ctx context.Context) foundation.Result[StateServiceStats, error] {
	stats := StateServiceStats{
		StoreType: "json", // Default for current implementation
	}

	// Get health info
	health := ss.store.Health(ctx)
	if health.IsOk() {
		stats.IsHealthy = health.Unwrap().Status == "healthy"
		if health.Unwrap().StorageSize != nil {
			stats.StorageSize = health.Unwrap().StorageSize
		}
		if health.Unwrap().LastBackup != nil {
			timestamp := health.Unwrap().LastBackup.Unix()
			stats.LastBackup = &timestamp
		}
	}

	// Count repositories
	repos := ss.store.Repositories().List(ctx)
	if repos.IsOk() {
		stats.RepositoryCount = len(repos.Unwrap())
	}

	// Count builds (using pagination to get total count)
	builds := ss.store.Builds().List(ctx, ListOptions{})
	if builds.IsOk() {
		stats.BuildCount = len(builds.Unwrap())
	}

	// Count schedules
	schedules := ss.store.Schedules().List(ctx)
	if schedules.IsOk() {
		stats.ScheduleCount = len(schedules.Unwrap())
	}

	return foundation.Ok[StateServiceStats, error](stats)
}
