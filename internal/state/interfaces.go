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

// Store is the main interface that aggregates all storage concerns.
// This replaces the monolithic StateManager with focused, composable stores.
type Store interface {
	// Repository operations
	Repositories() RepositoryStore

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
