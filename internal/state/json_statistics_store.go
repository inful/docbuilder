package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// jsonStatisticsStore implements StatisticsStore for the JSON store.
type jsonStatisticsStore struct {
	store *JSONStore
}

func (ss *jsonStatisticsStore) Get(_ context.Context) foundation.Result[*Statistics, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	statsCopy := *ss.store.statistics
	return foundation.Ok[*Statistics, error](&statsCopy)
}

func (ss *jsonStatisticsStore) Update(_ context.Context, stats *Statistics) foundation.Result[*Statistics, error] {
	if stats == nil {
		return foundation.Err[*Statistics, error](
			errors.ValidationError("statistics cannot be nil").Build(),
		)
	}

	return updateSimpleEntity[Statistics](
		ss.store,
		stats,
		func() { stats.LastUpdated = time.Now() },
		func() { ss.store.statistics = stats },
	)
}

func (ss *jsonStatisticsStore) RecordDiscovery(_ context.Context, documentCount int) foundation.Result[struct{}, error] {
	if documentCount < 0 {
		return foundation.Err[struct{}, error](
			errors.ValidationError("document count cannot be negative").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	ss.store.statistics.TotalDiscoveries++
	ss.store.statistics.DocumentsFound += int64(documentCount)
	ss.store.statistics.LastUpdated = time.Now()

	ss.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}

func (ss *jsonStatisticsStore) Reset(_ context.Context) foundation.Result[struct{}, error] {
	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	now := time.Now()
	ss.store.statistics = &Statistics{
		LastStatReset: now,
		LastUpdated:   now,
	}

	ss.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}
