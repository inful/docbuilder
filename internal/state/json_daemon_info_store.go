package state

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonDaemonInfoStore implements DaemonInfoStore for the JSON store.
type jsonDaemonInfoStore struct {
	store *JSONStore
}

func (ds *jsonDaemonInfoStore) Get(_ context.Context) foundation.Result[*DaemonInfo, error] {
	ds.store.mu.RLock()
	defer ds.store.mu.RUnlock()

	infoCopy := *ds.store.daemonInfo
	return foundation.Ok[*DaemonInfo, error](&infoCopy)
}

func (ds *jsonDaemonInfoStore) Update(_ context.Context, info *DaemonInfo) foundation.Result[*DaemonInfo, error] {
	if info == nil {
		return foundation.Err[*DaemonInfo, error](
			foundation.ValidationError("daemon info cannot be nil").Build(),
		)
	}

	ds.store.mu.Lock()
	defer ds.store.mu.Unlock()

	info.LastUpdate = time.Now()
	ds.store.daemonInfo = info

	if ds.store.autoSaveEnabled {
		if err := ds.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*DaemonInfo, error](
				foundation.InternalError("failed to save daemon info").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*DaemonInfo, error](info)
}

func (ds *jsonDaemonInfoStore) UpdateStatus(_ context.Context, status string) foundation.Result[struct{}, error] {
	if status == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("status cannot be empty").Build(),
		)
	}

	ds.store.mu.Lock()
	defer ds.store.mu.Unlock()

	ds.store.daemonInfo.Status = status
	ds.store.daemonInfo.LastUpdate = time.Now()

	if ds.store.autoSaveEnabled {
		if err := ds.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save daemon status").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}
