package state

import (
	"context"
	"maps"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// jsonConfigurationStore implements ConfigurationStore for the JSON store.
type jsonConfigurationStore struct {
	store *JSONStore
}

func (cs *jsonConfigurationStore) Get(_ context.Context, key string) foundation.Result[foundation.Option[any], error] {
	if key == "" {
		return foundation.Err[foundation.Option[any], error](
			errors.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.RLock()
	defer cs.store.mu.RUnlock()

	if value, exists := cs.store.configuration[key]; exists {
		return foundation.Ok[foundation.Option[any], error](foundation.Some(value))
	}

	return foundation.Ok[foundation.Option[any], error](foundation.None[any]())
}

func (cs *jsonConfigurationStore) Set(_ context.Context, key string, value any) foundation.Result[struct{}, error] {
	if key == "" {
		return foundation.Err[struct{}, error](
			errors.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.Lock()
	defer cs.store.mu.Unlock()

	cs.store.configuration[key] = value

	if cs.store.autoSaveEnabled {
		if err := cs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				errors.InternalError("failed to save configuration").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (cs *jsonConfigurationStore) Delete(_ context.Context, key string) foundation.Result[struct{}, error] {
	cs.store.mu.Lock()
	defer cs.store.mu.Unlock()

	return deleteEntity(
		key,
		func() bool { _, exists := cs.store.configuration[key]; return exists },
		func() { delete(cs.store.configuration, key) },
		func() error {
			if cs.store.autoSaveEnabled {
				return cs.store.saveToDiskUnsafe()
			}
			return nil
		},
		"configuration key",
		"failed to save configuration deletion",
	)
}

func (cs *jsonConfigurationStore) List(ctx context.Context) foundation.Result[map[string]any, error] {
	return cs.GetAll(ctx)
}

func (cs *jsonConfigurationStore) GetAll(_ context.Context) foundation.Result[map[string]any, error] {
	cs.store.mu.RLock()
	defer cs.store.mu.RUnlock()

	// Return a deep copy to prevent external modification
	result := make(map[string]any, len(cs.store.configuration))
	maps.Copy(result, cs.store.configuration)

	return foundation.Ok[map[string]any, error](result)
}
