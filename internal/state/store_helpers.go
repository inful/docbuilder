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

// ptrValidatable constrains P to be a pointer to T and also Validatable.
// This allows nil checks in generic helpers.
type ptrValidatable[T any] interface {
	~*T
	Validatable
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

// updateValidatableEntity is a generic helper for update flows where:
// - entity is pointer-like and can be nil
// - entity validates itself via Validate()
// - UpdatedAt should be bumped on successful update
//
// It centralizes the common patterns used across Build/Repository/Schedule stores,
// avoiding repeated code that triggers the dupl linter.
func updateValidatableEntity[T any, P ptrValidatable[T]](
	js *JSONStore,
	entityName string,
	entity P,
	exists func() bool,
	setUpdatedAt func(),
	write func(),
	onNotFound func() foundation.Result[P, error],
	saveErrMsg string,
) foundation.Result[P, error] {
	if entity == nil {
		return foundation.Err[P, error](
			errors.ValidationError(entityName + " cannot be nil").Build(),
		)
	}

	if validationResult := entity.Validate(); !validationResult.Valid {
		return foundation.Err[P, error](validationResult.ToError())
	}

	js.mu.Lock()
	defer js.mu.Unlock()

	if !exists() {
		return onNotFound()
	}

	setUpdatedAt()

	write()

	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[P, error](
				errors.InternalError(saveErrMsg).WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[P, error](entity)
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
