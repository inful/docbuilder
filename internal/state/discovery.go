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
