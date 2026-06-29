package state

import (
	"context"
)

// IncrementRepoBuild increments build counters for a repository.
func (a *ServiceAdapter) IncrementRepoBuild(url string, success bool) {
	if url == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetRepositoryStore()
	_ = store.IncrementBuildCount(ctx, url, success)
}
