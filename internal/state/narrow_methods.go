package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// This file implements the narrow interfaces declared in narrow_interfaces.go
// (RepositoryInitializer, RepositoryMetadataWriter, RepositoryMetadataReader,
// RepositoryMetadataStore, RepositoryCommitTracker, ConfigurationStateStore,
// DiscoveryRecorder) as methods on *Service. The implementations directly
// delegate to the inlined *JSONStore methods (RepositoryGetByURL,
// RepositoryUpdate, ConfigurationGet, etc).

// --- RepositoryInitializer ---

// EnsureRepositoryState creates a repository entry if it doesn't exist.
// For compatibility with legacy code, empty branch defaults to "main".
func (s *Service) EnsureRepositoryState(url, name, branch string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := s.store

	// Check if repository already exists
	existing := store.RepositoryGetByURL(ctx, url)
	if existing.IsOk() {
		// Already exists; nothing to do.
		return
	}

	// Default branch for compatibility with legacy code that passes empty branch
	if branch == "" {
		branch = defaultBranchMain
	}

	repo := &Repository{
		URL:       url,
		Name:      name,
		Branch:    branch,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = store.RepositoryCreate(ctx, repo) // Ignore error for interface compatibility
}

// RemoveRepositoryState removes a repository entry from persistent state.
// It is used by daemon orchestration to reflect discovery removals.
func (s *Service) RemoveRepositoryState(url string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	res := s.store.RepositoryDelete(ctx, url)
	if res.IsErr() {
		// Repository not found is acceptable here (idempotent removal).
		// Anything else is logged but not propagated — the orchestrator
		// tolerates best-effort cleanup.
		_ = res.UnwrapErr()
	}
}

// --- RepositoryMetadataWriter ---

// SetRepoDocumentCount sets the document count for a repository.
func (s *Service) SetRepoDocumentCount(url string, count int) {
	if url == "" || count < 0 {
		return
	}
	ctx := context.Background()
	_ = s.store.RepositorySetDocumentCount(ctx, url, count)
}

// SetRepoDocFilesHash sets the document files hash for a repository.
func (s *Service) SetRepoDocFilesHash(url, hash string) {
	if url == "" || hash == "" {
		return
	}
	ctx := context.Background()
	_ = s.store.RepositorySetDocFilesHash(ctx, url, hash)
}

// SetRepoDocFilePaths sets the document file paths for a repository.
func (s *Service) SetRepoDocFilePaths(url string, paths []string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	_ = s.store.RepositorySetDocFilePaths(ctx, url, paths)
}

// --- RepositoryMetadataReader ---

// GetRepoDocFilesHash returns the document files hash for a repository.
func (s *Service) GetRepoDocFilesHash(url string) string {
	if url == "" {
		return ""
	}
	ctx := context.Background()
	result := s.store.RepositoryGetByURL(ctx, url)
	if result.IsErr() {
		return ""
	}
	repo := result.Unwrap()
	if repo == nil {
		return ""
	}
	hashOpt := repo.DocFilesHash
	if hashOpt.IsNone() {
		return ""
	}
	return hashOpt.Unwrap()
}

// GetRepoDocFilePaths returns the document file paths for a repository.
func (s *Service) GetRepoDocFilePaths(url string) []string {
	if url == "" {
		return nil
	}
	ctx := context.Background()
	result := s.store.RepositoryGetByURL(ctx, url)
	if result.IsErr() {
		return nil
	}
	repo := result.Unwrap()
	if repo == nil {
		return nil
	}
	// Return a copy to prevent mutation
	paths := repo.DocFilePaths
	if len(paths) == 0 {
		return nil
	}
	cp := make([]string, len(paths))
	copy(cp, paths)
	return cp
}

// --- RepositoryCommitTracker ---

// SetRepoLastCommit stores the last commit hash for a repository.
func (s *Service) SetRepoLastCommit(url, name, branch, commit string) {
	if url == "" || commit == "" {
		return
	}
	ctx := context.Background()
	store := s.store

	// Get existing repository to update
	result := store.RepositoryGetByURL(ctx, url)
	if result.IsErr() {
		return
	}
	repo := result.Unwrap()
	if repo == nil {
		// Repository doesn't exist, create it first
		s.EnsureRepositoryState(url, name, branch)
		result = store.RepositoryGetByURL(ctx, url)
		if result.IsErr() || result.Unwrap() == nil {
			return
		}
		repo = result.Unwrap()
	}

	// Update the repository with the new commit
	repo.LastCommit = foundation.Some(commit)
	repo.UpdatedAt = time.Now()
	_ = store.RepositoryUpdate(ctx, repo)
}

// GetRepoLastCommit returns the last commit hash for a repository.
func (s *Service) GetRepoLastCommit(url string) string {
	if url == "" {
		return ""
	}
	ctx := context.Background()
	result := s.store.RepositoryGetByURL(ctx, url)
	if result.IsErr() {
		return ""
	}
	repo := result.Unwrap()
	if repo == nil {
		return ""
	}
	commitOpt := repo.LastCommit
	if commitOpt.IsNone() {
		return ""
	}
	return commitOpt.Unwrap()
}

// --- RepositoryBuildCounter ---

// IncrementRepoBuild increments build counters for a repository.
func (s *Service) IncrementRepoBuild(url string, success bool) {
	if url == "" {
		return
	}
	ctx := context.Background()
	_ = s.store.RepositoryIncrementBuildCount(ctx, url, success)
}

// --- ConfigurationStateStore ---

// SetLastConfigHash stores the last config hash.
func (s *Service) SetLastConfigHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	_ = s.store.ConfigurationSet(ctx, "last_config_hash", hash)
}

// GetLastConfigHash returns the last config hash.
func (s *Service) GetLastConfigHash() string {
	ctx := context.Background()
	result := s.store.ConfigurationGet(ctx, "last_config_hash")
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
func (s *Service) SetLastReportChecksum(sum string) {
	if sum == "" {
		return
	}
	ctx := context.Background()
	_ = s.store.ConfigurationSet(ctx, "last_report_checksum", sum)
}

// GetLastReportChecksum returns the last report checksum.
func (s *Service) GetLastReportChecksum() string {
	ctx := context.Background()
	result := s.store.ConfigurationGet(ctx, "last_report_checksum")
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
func (s *Service) SetLastGlobalDocFilesHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	_ = s.store.ConfigurationSet(ctx, "last_global_doc_files_hash", hash)
}

// GetLastGlobalDocFilesHash returns the global doc files hash.
func (s *Service) GetLastGlobalDocFilesHash() string {
	ctx := context.Background()
	result := s.store.ConfigurationGet(ctx, "last_global_doc_files_hash")
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

// --- DiscoveryRecorder ---

// RecordDiscovery records a discovery operation for a repository.
func (s *Service) RecordDiscovery(repoURL string, documentCount int) {
	if repoURL == "" {
		return
	}
	ctx := context.Background()

	// Update repository state. (Statistics tracking is gone; the
	// StatisticsStore was deleted as dead code in M12.)
	repoStore := s.store
	result := repoStore.RepositoryGetByURL(ctx, repoURL)
	if result.IsOk() {
		repo := result.Unwrap()
		if repo != nil {
			now := time.Now()
			repo.LastDiscovery = foundation.Some(now)
			repo.DocumentCount = documentCount
			repo.UpdatedAt = now
			_ = repoStore.RepositoryUpdate(ctx, repo)
		}
	}
}

// --- Test helper ---

// GetRepository returns the stored repository state for url, or nil if
// not found. It exposes the internal *Repository type so callers (notably
// the daemon integration tests) can inspect fields directly.
func (s *Service) GetRepository(url string) *Repository {
	if url == "" {
		return nil
	}
	ctx := context.Background()
	result := s.store.RepositoryGetByURL(ctx, url)
	if result.IsErr() {
		return nil
	}
	return result.Unwrap()
}
