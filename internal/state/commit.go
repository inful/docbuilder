package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

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
