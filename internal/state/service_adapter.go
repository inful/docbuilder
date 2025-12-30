package state

import (
	"context"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// ServiceAdapter wraps a state.Service and implements the narrow interfaces
// defined in narrow_interfaces.go (DaemonStateManager). This is the canonical
// implementation for daemon state management.
//
// The adapter translates simple method signatures (no context, no Result types)
// to the typed Store method signatures (context + Result types).
type ServiceAdapter struct {
	service *Service
	mu      sync.RWMutex

	// Cached values for lifecycle methods
	loaded    bool
	lastSaved *time.Time
}

// NewServiceAdapter creates an adapter that wraps a state.Service.
func NewServiceAdapter(svc *Service) *ServiceAdapter {
	return &ServiceAdapter{
		service: svc,
		loaded:  false,
	}
}

// --- LifecycleManager interface ---

// Load loads state from the underlying store.
func (a *ServiceAdapter) Load() error {
	ctx := context.Background()
	// The typed store loads on creation, so this is mostly a health check
	health := a.service.Store().Health(ctx)
	if health.IsErr() {
		return health.UnwrapErr()
	}
	if health.Unwrap().Status != "healthy" {
		return errors.InternalError("state store unhealthy").
			WithContext("status", health.Unwrap().Status).
			Build()
	}
	a.mu.Lock()
	a.loaded = true
	a.mu.Unlock()
	return nil
}

// Save persists state to the underlying store.
func (a *ServiceAdapter) Save() error {
	ctx := context.Background()
	// The typed JSON store auto-saves, but we trigger a close/reopen cycle
	// for explicit save semantics, or just mark save time.
	// For now, just update the save timestamp since JSONStore auto-persists.
	a.mu.Lock()
	now := time.Now()
	a.lastSaved = &now
	a.mu.Unlock()

	// Optionally flush by checking health (which internally persists)
	health := a.service.Store().Health(ctx)
	if health.IsErr() {
		return health.UnwrapErr()
	}
	return nil
}

// IsLoaded returns whether the state has been loaded.
func (a *ServiceAdapter) IsLoaded() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.loaded
}

// LastSaved returns the last save timestamp.
func (a *ServiceAdapter) LastSaved() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastSaved
}

// --- RepositoryInitializer interface ---

// EnsureRepositoryState creates a repository entry if it doesn't exist.
// For compatibility with legacy code, empty branch defaults to "main".
func (a *ServiceAdapter) EnsureRepositoryState(url, name, branch string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()

	// Check if repository already exists
	existing := store.GetByURL(ctx, url)
	if existing.IsOk() {
		if opt := existing.Unwrap(); opt.IsSome() {
			return // Already exists
		}
	}

	// Default branch for compatibility with legacy code that passes empty branch
	if branch == "" {
		branch = "main"
	}

	// Create new repository
	repo := &Repository{
		URL:       url,
		Name:      name,
		Branch:    branch,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = store.Create(ctx, repo) // Ignore error for interface compatibility
}

// --- RepositoryMetadataWriter interface ---

// SetRepoDocumentCount sets the document count for a repository.
func (a *ServiceAdapter) SetRepoDocumentCount(url string, count int) {
	if url == "" || count < 0 {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	_ = store.SetDocumentCount(ctx, url, count)
}

// SetRepoDocFilesHash sets the document files hash for a repository.
func (a *ServiceAdapter) SetRepoDocFilesHash(url, hash string) {
	if url == "" || hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	_ = store.SetDocFilesHash(ctx, url, hash)
}

// --- RepositoryMetadataReader interface ---

// GetRepoDocFilesHash returns the document files hash for a repository.
func (a *ServiceAdapter) GetRepoDocFilesHash(url string) string {
	if url == "" {
		return ""
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	result := store.GetByURL(ctx, url)
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	hashOpt := opt.Unwrap().DocFilesHash
	if hashOpt.IsNone() {
		return ""
	}
	return hashOpt.Unwrap()
}

// GetRepoDocFilePaths returns the document file paths for a repository.
func (a *ServiceAdapter) GetRepoDocFilePaths(url string) []string {
	if url == "" {
		return nil
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	result := store.GetByURL(ctx, url)
	if result.IsErr() {
		return nil
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return nil
	}
	// Return a copy to prevent mutation
	paths := opt.Unwrap().DocFilePaths
	if len(paths) == 0 {
		return nil
	}
	cp := make([]string, len(paths))
	copy(cp, paths)
	return cp
}

// --- RepositoryMetadataStore interface (extends Reader + Writer) ---

// SetRepoDocFilePaths sets the document file paths for a repository.
func (a *ServiceAdapter) SetRepoDocFilePaths(url string, paths []string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	_ = store.SetDocFilePaths(ctx, url, paths)
}

// --- RepositoryCommitTracker interface ---

// SetRepoLastCommit sets the last commit hash for a repository.
func (a *ServiceAdapter) SetRepoLastCommit(url, name, branch, commit string) {
	if url == "" || commit == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()

	// Get existing repository to update
	result := store.GetByURL(ctx, url)
	if result.IsErr() {
		return
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		// Repository doesn't exist, create it first
		a.EnsureRepositoryState(url, name, branch)
	}

	// Update the repository with the new commit
	result = store.GetByURL(ctx, url)
	if result.IsErr() || result.Unwrap().IsNone() {
		return
	}
	repo := result.Unwrap().Unwrap()
	repo.LastCommit = foundation.Some(commit)
	repo.UpdatedAt = time.Now()
	_ = store.Update(ctx, repo)
}

// GetRepoLastCommit returns the last commit hash for a repository.
func (a *ServiceAdapter) GetRepoLastCommit(url string) string {
	if url == "" {
		return ""
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	result := store.GetByURL(ctx, url)
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	commitOpt := opt.Unwrap().LastCommit
	if commitOpt.IsNone() {
		return ""
	}
	return commitOpt.Unwrap()
}

// --- RepositoryBuildCounter interface ---

// IncrementRepoBuild increments build counters for a repository.
func (a *ServiceAdapter) IncrementRepoBuild(url string, success bool) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	_ = store.IncrementBuildCount(ctx, url, success)
}

// --- ConfigurationStateStore interface ---

// SetLastConfigHash stores the last config hash.
func (a *ServiceAdapter) SetLastConfigHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	_ = store.Set(ctx, "last_config_hash", hash)
}

// GetLastConfigHash returns the last config hash.
func (a *ServiceAdapter) GetLastConfigHash() string {
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	result := store.Get(ctx, "last_config_hash")
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	if s, ok := opt.Unwrap().(string); ok {
		return s
	}
	return ""
}

// SetLastReportChecksum stores the last report checksum.
func (a *ServiceAdapter) SetLastReportChecksum(sum string) {
	if sum == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	_ = store.Set(ctx, "last_report_checksum", sum)
}

// GetLastReportChecksum returns the last report checksum.
func (a *ServiceAdapter) GetLastReportChecksum() string {
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	result := store.Get(ctx, "last_report_checksum")
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	if s, ok := opt.Unwrap().(string); ok {
		return s
	}
	return ""
}

// SetLastGlobalDocFilesHash stores the global doc files hash.
func (a *ServiceAdapter) SetLastGlobalDocFilesHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	_ = store.Set(ctx, "last_global_doc_files_hash", hash)
}

// GetLastGlobalDocFilesHash returns the global doc files hash.
func (a *ServiceAdapter) GetLastGlobalDocFilesHash() string {
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	result := store.Get(ctx, "last_global_doc_files_hash")
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	if s, ok := opt.Unwrap().(string); ok {
		return s
	}
	return ""
}

// --- Additional methods used by Daemon ---

// RecordDiscovery records a discovery operation for a repository.
// This mimics the legacy StateManager.RecordDiscovery method.
func (a *ServiceAdapter) RecordDiscovery(repoURL string, documentCount int) {
	if repoURL == "" {
		return
	}
	ctx := context.Background()

	// Update statistics using RecordDiscovery method
	statsStore := a.service.GetStatisticsStore()
	_ = statsStore.RecordDiscovery(ctx, documentCount)

	// Update repository state
	repoStore := a.service.GetRepositoryStore()
	result := repoStore.GetByURL(ctx, repoURL)
	if result.IsOk() {
		if opt := result.Unwrap(); opt.IsSome() {
			repo := opt.Unwrap()
			now := time.Now()
			repo.LastDiscovery = foundation.Some(now)
			repo.DocumentCount = documentCount
			repo.UpdatedAt = now
			_ = repoStore.Update(ctx, repo)
		}
	}
}

// --- Test helper methods ---

// RepositoryState is a simplified view of repository state for test compatibility.
// Uses plain types instead of foundation.Option for easier test assertions.
type RepositoryState struct {
	URL           string
	Name          string
	Branch        string
	LastDiscovery *time.Time
	LastBuild     *time.Time
	LastCommit    string
	DocumentCount int
	BuildCount    int64
	ErrorCount    int64
	LastError     string
	DocFilesHash  string
	DocFilePaths  []string
}

// GetRepository retrieves repository state by URL, returning nil if not found.
// This is a convenience method primarily for test assertions.
func (a *ServiceAdapter) GetRepository(url string) *RepositoryState {
	if url == "" {
		return nil
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	result := store.GetByURL(ctx, url)
	if result.IsErr() {
		return nil
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return nil
	}
	repo := opt.Unwrap()

	// Convert from state.Repository to RepositoryState
	rs := &RepositoryState{
		URL:           repo.URL,
		Name:          repo.Name,
		Branch:        repo.Branch,
		DocumentCount: repo.DocumentCount,
		BuildCount:    repo.BuildCount,
		ErrorCount:    repo.ErrorCount,
		DocFilePaths:  repo.DocFilePaths,
	}

	// Convert Option types to plain types/pointers
	if repo.LastDiscovery.IsSome() {
		t := repo.LastDiscovery.Unwrap()
		rs.LastDiscovery = &t
	}
	if repo.LastBuild.IsSome() {
		t := repo.LastBuild.Unwrap()
		rs.LastBuild = &t
	}
	if repo.LastCommit.IsSome() {
		rs.LastCommit = repo.LastCommit.Unwrap()
	}
	if repo.LastError.IsSome() {
		rs.LastError = repo.LastError.Unwrap()
	}
	if repo.DocFilesHash.IsSome() {
		rs.DocFilesHash = repo.DocFilesHash.Unwrap()
	}

	return rs
}

// Compile-time verification that ServiceAdapter implements DaemonStateManager.
var _ DaemonStateManager = (*ServiceAdapter)(nil)
