package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// RepositoryStore handles repository state persistence and queries.
type RepositoryStore interface {
	// Create creates a new repository record.
	Create(ctx context.Context, repo *Repository) foundation.Result[*Repository, error]

	// GetByURL retrieves a repository by its URL.
	GetByURL(ctx context.Context, url string) foundation.Result[foundation.Option[*Repository], error]

	// Update updates an existing repository.
	Update(ctx context.Context, repo *Repository) foundation.Result[*Repository, error]

	// List returns all repositories with optional filtering.
	List(ctx context.Context) foundation.Result[[]Repository, error]

	// Delete removes a repository by URL.
	Delete(ctx context.Context, url string) foundation.Result[struct{}, error]

	// IncrementBuildCount increments build counters for a repository.
	IncrementBuildCount(ctx context.Context, url string, success bool) foundation.Result[struct{}, error]

	// SetDocumentCount updates the document count for a repository.
	SetDocumentCount(ctx context.Context, url string, count int) foundation.Result[struct{}, error]

	// SetDocFilesHash updates the document files hash for incremental detection.
	SetDocFilesHash(ctx context.Context, url string, hash string) foundation.Result[struct{}, error]

	// SetDocFilePaths updates the document file paths for a repository.
	SetDocFilePaths(ctx context.Context, url string, paths []string) foundation.Result[struct{}, error]
}

// BuildStore handles build state persistence and queries.
type BuildStore interface {
	// Create creates a new build record.
	Create(ctx context.Context, build *Build) foundation.Result[*Build, error]

	// GetByID retrieves a build by its ID.
	GetByID(ctx context.Context, id string) foundation.Result[foundation.Option[*Build], error]

	// Update updates an existing build.
	Update(ctx context.Context, build *Build) foundation.Result[*Build, error]

	// List returns builds with optional filtering and pagination.
	List(ctx context.Context, opts ListOptions) foundation.Result[[]Build, error]

	// Delete removes a build by ID.
	Delete(ctx context.Context, id string) foundation.Result[struct{}, error]

	// Cleanup removes old builds to prevent unbounded storage growth.
	Cleanup(ctx context.Context, maxBuilds int) foundation.Result[int, error]
}

// ScheduleStore handles schedule state persistence and queries.
type ScheduleStore interface {
	// Create creates a new schedule.
	Create(ctx context.Context, schedule *Schedule) foundation.Result[*Schedule, error]

	// GetByID retrieves a schedule by its ID.
	GetByID(ctx context.Context, id string) foundation.Result[foundation.Option[*Schedule], error]

	// Update updates an existing schedule.
	Update(ctx context.Context, schedule *Schedule) foundation.Result[*Schedule, error]

	// List returns all schedules.
	List(ctx context.Context) foundation.Result[[]Schedule, error]

	// Delete removes a schedule by ID.
	Delete(ctx context.Context, id string) foundation.Result[struct{}, error]
}

// StatisticsStore handles statistics persistence and calculations.
type StatisticsStore interface {
	// Get retrieves current statistics.
	Get(ctx context.Context) foundation.Result[*Statistics, error]

	// Update updates statistics.
	Update(ctx context.Context, stats *Statistics) foundation.Result[*Statistics, error]

	// RecordBuild updates statistics when a build completes.
	RecordBuild(ctx context.Context, build *Build) foundation.Result[struct{}, error]

	// RecordDiscovery updates statistics when a discovery completes.
	RecordDiscovery(ctx context.Context, documentCount int) foundation.Result[struct{}, error]

	// Reset resets all statistics counters.
	Reset(ctx context.Context) foundation.Result[struct{}, error]
}

// ConfigurationStore handles configuration data persistence.
type ConfigurationStore interface {
	// Set stores a configuration value.
	Set(ctx context.Context, key string, value any) foundation.Result[struct{}, error]

	// Get retrieves a configuration value.
	Get(ctx context.Context, key string) foundation.Result[foundation.Option[any], error]

	// Delete removes a configuration key.
	Delete(ctx context.Context, key string) foundation.Result[struct{}, error]

	// List returns all configuration keys and values.
	List(ctx context.Context) foundation.Result[map[string]any, error]
}

// DaemonInfoStore handles daemon metadata persistence.
type DaemonInfoStore interface {
	// Get retrieves daemon information.
	Get(ctx context.Context) foundation.Result[*DaemonInfo, error]

	// Update updates daemon information.
	Update(ctx context.Context, info *DaemonInfo) foundation.Result[*DaemonInfo, error]

	// UpdateStatus updates only the daemon status.
	UpdateStatus(ctx context.Context, status string) foundation.Result[struct{}, error]
}

// ListOptions provides filtering and pagination options for list operations.
type ListOptions struct {
	Limit  foundation.Option[int]    `json:"limit"`
	Offset foundation.Option[int]    `json:"offset"`
	SortBy foundation.Option[string] `json:"sort_by"`
	Order  foundation.Option[string] `json:"order"` // "asc" or "desc"
	Filter map[string]any            `json:"filter"`
}

// Store is the main interface that aggregates all storage concerns.
// This replaces the monolithic StateManager with focused, composable stores.
type Store interface {
	// Repository operations
	Repositories() RepositoryStore

	// Build operations
	Builds() BuildStore

	// Schedule operations
	Schedules() ScheduleStore

	// Statistics operations
	Statistics() StatisticsStore

	// Configuration operations
	Configuration() ConfigurationStore

	// Daemon info operations
	DaemonInfo() DaemonInfoStore

	// Transaction operations
	WithTransaction(ctx context.Context, fn func(Store) error) foundation.Result[struct{}, error]

	// Health and lifecycle
	Health(ctx context.Context) foundation.Result[StoreHealth, error]
	Close(ctx context.Context) foundation.Result[struct{}, error]
}

// StoreHealth represents the health status of the state store.
type StoreHealth struct {
	Status      string     `json:"status"`
	Message     string     `json:"message,omitempty"`
	LastBackup  *time.Time `json:"last_backup,omitempty"`
	StorageSize *int64     `json:"storage_size_bytes,omitempty"`
	CheckedAt   time.Time  `json:"checked_at"`
}
