package state

import (
	"sync"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// Validatable represents an entity that can be validated.
type Validatable interface {
	Validate() foundation.ValidationResult
}

// createEntity is a generic helper for creating entities with timestamp setting and auto-save.
// T must be a pointer type to an entity with ID, CreatedAt, and UpdatedAt fields.
func createEntity[T Validatable](
	entity T,
	entityName string,
	mu *sync.RWMutex,
	setTimestamps func(T),
	addToStore func(T),
	removeFromStore func(T),
	autoSaveEnabled bool,
	saveToDisk func() error,
) foundation.Result[T, error] {
	if validationResult := entity.Validate(); !validationResult.Valid {
		return foundation.Err[T, error](validationResult.ToError())
	}

	mu.Lock()
	defer mu.Unlock()

	setTimestamps(entity)
	addToStore(entity)

	if autoSaveEnabled {
		if err := saveToDisk(); err != nil {
			removeFromStore(entity)
			return foundation.Err[T, error](
				errors.InternalError("failed to save " + entityName).WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[T, error](entity)
}

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
				errors.InternalError(saveErrMsg).WithCause(err).Build(),
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
				errors.InternalError(saveErrMsg).WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*T, error](obj)
}
