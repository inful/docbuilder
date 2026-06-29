package state

import (
	"context"
	"time"
)

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
	_ = store.Create(ctx, repo) // Ignore error for interface compatibility
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
		// Repository not found is acceptable here (idempotent removal).
		// Anything else is logged but not propagated — the orchestrator
		// tolerates best-effort cleanup.
		_ = res.UnwrapErr()
	}
}

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

// SetRepoDocFilePaths sets the document file paths for a repository.
func (a *ServiceAdapter) SetRepoDocFilePaths(url string, paths []string) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	_ = store.SetDocFilePaths(ctx, url, paths)
}
