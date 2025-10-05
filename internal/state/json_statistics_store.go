package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonStatisticsStore implements StatisticsStore for the JSON store.
type jsonStatisticsStore struct {
	store *JSONStore
}

func (ss *jsonStatisticsStore) Get(ctx context.Context) foundation.Result[*Statistics, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	statsCopy := *ss.store.statistics
	return foundation.Ok[*Statistics, error](&statsCopy)
}

func (ss *jsonStatisticsStore) Update(ctx context.Context, stats *Statistics) foundation.Result[*Statistics, error] {
	if stats == nil {
		return foundation.Err[*Statistics, error](
			foundation.ValidationError("statistics cannot be nil").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	stats.LastUpdated = time.Now()
	ss.store.statistics = stats

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Statistics, error](
				foundation.InternalError("failed to save statistics").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Statistics, error](stats)
}

func (ss *jsonStatisticsStore) RecordBuild(ctx context.Context, build *Build) foundation.Result[struct{}, error] {
	if build == nil {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("build cannot be nil").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	ss.store.statistics.TotalBuilds++
	if build.Status == BuildStatusCompleted {
		ss.store.statistics.SuccessfulBuilds++
	} else if build.Status == BuildStatusFailed {
		ss.store.statistics.FailedBuilds++
	}
	ss.store.statistics.LastUpdated = time.Now()

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save build statistics").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (ss *jsonStatisticsStore) RecordDiscovery(ctx context.Context, documentCount int) foundation.Result[struct{}, error] {
	if documentCount < 0 {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("document count cannot be negative").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	ss.store.statistics.TotalDiscoveries++
	ss.store.statistics.DocumentsFound += int64(documentCount)
	ss.store.statistics.LastUpdated = time.Now()

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save discovery statistics").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (ss *jsonStatisticsStore) Reset(ctx context.Context) foundation.Result[struct{}, error] {
	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	now := time.Now()
	ss.store.statistics = &Statistics{
		LastStatReset: now,
		LastUpdated:   now,
	}

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save statistics reset").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}
