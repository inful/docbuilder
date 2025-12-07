// Package state provides interfaces and types for state management.
// This file defines narrow, composable interfaces that bridge legacy and typed state implementations.
package state

import "time"

// RepositoryMetadataWriter provides methods to update repository discovery/build metadata.
// This is the minimal interface Hugo generator needs for state persistence.
// Implemented by state.ServiceAdapter (the canonical implementation).
type RepositoryMetadataWriter interface {
	// SetRepoDocumentCount stores the document count discovered for a repository.
	SetRepoDocumentCount(url string, count int)

	// SetRepoDocFilesHash stores the computed hash of document files for change detection.
	SetRepoDocFilesHash(url string, hash string)
}

// RepositoryMetadataReader provides methods to read repository discovery metadata.
type RepositoryMetadataReader interface {
	// GetRepoDocFilesHash returns the stored document files hash for a repository.
	GetRepoDocFilesHash(url string) string

	// GetRepoDocFilePaths returns the stored document file paths for a repository.
	GetRepoDocFilePaths(url string) []string
}

// RepositoryMetadataStore combines read and write operations for repository metadata.
type RepositoryMetadataStore interface {
	RepositoryMetadataReader
	RepositoryMetadataWriter

	// SetRepoDocFilePaths stores the sorted list of document file paths for a repository.
	SetRepoDocFilePaths(url string, paths []string)
}

// RepositoryInitializer ensures repository state entries exist before operations.
type RepositoryInitializer interface {
	// EnsureRepositoryState creates a repository state entry if it doesn't exist.
	EnsureRepositoryState(url, name, branch string)
}

// RepositoryCommitTracker tracks repository commit state for change detection.
type RepositoryCommitTracker interface {
	// SetRepoLastCommit updates the stored last commit hash for a repository.
	SetRepoLastCommit(url, name, branch, commit string)

	// GetRepoLastCommit returns the stored last commit hash for a repository.
	GetRepoLastCommit(url string) string
}

// RepositoryBuildCounter tracks build statistics for repositories.
type RepositoryBuildCounter interface {
	// IncrementRepoBuild increments build counters for a repository.
	IncrementRepoBuild(url string, success bool)
}

// ConfigurationStore stores daemon configuration state (hashes, checksums).
type ConfigurationStateStore interface {
	// SetLastConfigHash stores the hash of the last successful config.
	SetLastConfigHash(hash string)

	// GetLastConfigHash returns the stored config hash.
	GetLastConfigHash() string

	// SetLastReportChecksum stores the checksum of the last build report.
	SetLastReportChecksum(sum string)

	// GetLastReportChecksum returns the stored build report checksum.
	GetLastReportChecksum() string

	// SetLastGlobalDocFilesHash stores the global doc files hash.
	SetLastGlobalDocFilesHash(hash string)

	// GetLastGlobalDocFilesHash returns the stored global doc files hash.
	GetLastGlobalDocFilesHash() string
}

// LifecycleManager provides lifecycle operations for state managers.
// This mirrors services.StateManager for compatibility.
type LifecycleManager interface {
	Load() error
	Save() error
	IsLoaded() bool
	LastSaved() *time.Time
}

// DiscoveryRecorder records discovery operations for repositories.
type DiscoveryRecorder interface {
	// RecordDiscovery records a discovery operation for a repository.
	RecordDiscovery(repoURL string, documentCount int)
}

// DaemonStateManager is the aggregate interface for daemon state management.
// It combines all the narrow interfaces into a single type for convenient type assertions.
// Implemented by state.ServiceAdapter.
type DaemonStateManager interface {
	LifecycleManager
	RepositoryInitializer
	RepositoryMetadataStore
	RepositoryCommitTracker
	RepositoryBuildCounter
	ConfigurationStateStore
	DiscoveryRecorder
}

// Compile-time verification that ServiceAdapter implements DaemonStateManager.
var _ DaemonStateManager = (*ServiceAdapter)(nil)
