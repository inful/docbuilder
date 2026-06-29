package state

import (
	"context"
)

// GetRepository returns the stored repository state for url, or nil if not
// found. It exposes the internal *Repository type so callers (notably the
// daemon integration tests) can inspect fields directly.
func (a *ServiceAdapter) GetRepository(url string) *Repository {
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
	return opt.Unwrap()
}
