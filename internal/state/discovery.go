package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// RecordDiscovery records a discovery operation for a repository.
// This mimics the legacy StateManager.RecordDiscovery method.
func (a *ServiceAdapter) RecordDiscovery(repoURL string, documentCount int) {
	if repoURL == "" {
		return
	}
	ctx := context.Background()

	// Update repository state. Statistics tracking is gone (the
	// StatisticsStore was deleted as dead code in M12).
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
