package state

import (
	"context"
	"log/slog"
	"sync"
	"time"

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
	if health.Unwrap().Status != healthyStatus {
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
	res := a.service.Store().Flush(ctx)
	if res.IsErr() {
		return res.UnwrapErr()
	}

	a.mu.Lock()
	now := time.Now()
	a.lastSaved = &now
	a.mu.Unlock()
	return nil
}

// logStateError logs a state write that failed.
//
// The narrow interfaces the daemon uses cannot return an error, so a
// write that fails would be invisible: the daemon would carry on with
// a state that never reached the disk, and the next build would
// decide with stale data.
func logStateError(what string, err error) {
	slog.Warn("Failed to write the state",
		slog.String("value", what),
		slog.Any("error", err))
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
		branch = defaultBranchMain
	}

	// Create new repository
	repo := &Repository{
		URL:       url,
		Name:      name,
		Branch:    branch,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if res := store.Create(ctx, repo); res.IsErr() {
		logStateError("the repository", res.UnwrapErr())
	}
}

// RemoveRepositoryState removes a repository entry from persistent state.
// It is used by daemon orchestration to reflect discovery removals.
func (a *ServiceAdapter) RemoveRepositoryState(url string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	res := store.Delete(ctx, url)
	if res.IsErr() {
		slog.Warn("Failed to delete repository state",
			slog.String("url", url),
			slog.Any("error", res.UnwrapErr()))
	}
}

// --- RepositoryMetadataWriter interface ---

// SetRepoDocumentCount sets the document count for a repository.
func (a *ServiceAdapter) SetRepoDocumentCount(url string, count int) {
	if url == "" || count < 0 {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	if res := store.SetDocumentCount(ctx, url, count); res.IsErr() {
		logStateError("the document count", res.UnwrapErr())
	}
}

// SetRepoDocFilesHash sets the document files hash for a repository.
func (a *ServiceAdapter) SetRepoDocFilesHash(url, hash string) {
	if url == "" || hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	if res := store.SetDocFilesHash(ctx, url, hash); res.IsErr() {
		logStateError("the doc-files hash", res.UnwrapErr())
	}
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

	// Update the repository with the new commit. The store changes the
	// entry under its lock: a read of the entry and a write of it back
	// lose the changes that another writer made in between.
	if res := store.SetLastCommit(ctx, url, commit, time.Now()); res.IsErr() {
		logStateError("the commit", res.UnwrapErr())
	}
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
	if res := store.IncrementBuildCount(ctx, url, success); res.IsErr() {
		logStateError("the build counters", res.UnwrapErr())
	}
}

// RepositoryURLs returns the addresses of the repositories the state
// knows.
func (a *ServiceAdapter) RepositoryURLs() []string {
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	result := store.List(ctx)
	if result.IsErr() {
		return nil
	}
	repos := result.Unwrap()
	urls := make([]string, 0, len(repos))
	for i := range repos {
		urls = append(urls, repos[i].URL)
	}
	return urls
}

// --- ConfigurationStateStore interface ---

// SetLastConfigHash stores the last config hash.
func (a *ServiceAdapter) SetLastConfigHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	if res := store.Set(ctx, "last_config_hash", hash); res.IsErr() {
		logStateError("the config hash", res.UnwrapErr())
	}
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
	if res := store.Set(ctx, "last_report_checksum", sum); res.IsErr() {
		logStateError("the report checksum", res.UnwrapErr())
	}
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
	if res := store.Set(ctx, "last_global_doc_files_hash", hash); res.IsErr() {
		logStateError("the global doc-files hash", res.UnwrapErr())
	}
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

	// Update repository state. The store changes the entry under its
	// lock, so the discovery does not write over the commit that the
	// build recorded in the meantime.
	repoStore := a.service.GetRepositoryStore()
	if res := repoStore.SetLastDiscovery(ctx, repoURL, time.Now()); res.IsErr() {
		logStateError("the time of the last discovery", res.UnwrapErr())
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
