package state

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
			errors.ValidationError("daemon info cannot be nil").Build(),
		)
	}

	return updateSimpleEntity[DaemonInfo](
		ds.store,
		info,
		func() { info.LastUpdate = time.Now() },
		func() { ds.store.daemonInfo = info },
		"failed to save daemon info",
	)
}

func (ds *jsonDaemonInfoStore) UpdateStatus(_ context.Context, status string) foundation.Result[struct{}, error] {
	if status == "" {
		return foundation.Err[struct{}, error](
			errors.ValidationError("status cannot be empty").Build(),
		)
	}

	ds.store.mu.Lock()
	defer ds.store.mu.Unlock()

	ds.store.daemonInfo.Status = status
	ds.store.daemonInfo.LastUpdate = time.Now()

	if ds.store.autoSaveEnabled {
		if err := ds.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				errors.InternalError("failed to save daemon status").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}
