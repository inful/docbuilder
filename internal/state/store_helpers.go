package state

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// updateEntity centralizes the common update flow for JSON-backed stores.
// It assumes validation has already been performed by the caller.
func updateEntity[T any](
	js *JSONStore,
	obj *T,
	exists func() bool,
	setUpdatedAt func(),
	write func(),
	onNotFound func() foundation.Result[*T, error],
	saveErrMsg string,
) foundation.Result[*T, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	if !exists() {
		return onNotFound()
	}

	setUpdatedAt()
	write()

	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*T, error](
				foundation.InternalError(saveErrMsg).WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*T, error](obj)
}

// updateSimpleEntity is a variant for entities that don't track UpdatedAt
// (like DaemonInfo and Statistics that use their own timestamp fields).
func updateSimpleEntity[T any](
	js *JSONStore,
	obj *T,
	updateTimestamp func(),
	write func(),
	saveErrMsg string,
) foundation.Result[*T, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	updateTimestamp()
	write()

	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*T, error](
				foundation.InternalError(saveErrMsg).WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*T, error](obj)
}
