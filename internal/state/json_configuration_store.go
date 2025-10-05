package state

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonConfigurationStore implements ConfigurationStore for the JSON store.
type jsonConfigurationStore struct {
	store *JSONStore
}

func (cs *jsonConfigurationStore) Get(ctx context.Context, key string) foundation.Result[foundation.Option[any], error] {
	if key == "" {
		return foundation.Err[foundation.Option[any], error](
			foundation.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.RLock()
	defer cs.store.mu.RUnlock()

	if value, exists := cs.store.configuration[key]; exists {
		return foundation.Ok[foundation.Option[any], error](foundation.Some(value))
	}

	return foundation.Ok[foundation.Option[any], error](foundation.None[any]())
}

func (cs *jsonConfigurationStore) Set(ctx context.Context, key string, value any) foundation.Result[struct{}, error] {
	if key == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.Lock()
	defer cs.store.mu.Unlock()

	cs.store.configuration[key] = value

	if cs.store.autoSaveEnabled {
		if err := cs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save configuration").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (cs *jsonConfigurationStore) Delete(ctx context.Context, key string) foundation.Result[struct{}, error] {
	if key == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.Lock()
	defer cs.store.mu.Unlock()

	// Check if key exists
	if _, exists := cs.store.configuration[key]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("configuration key").
				WithContext(foundation.Fields{"key": key}).
				Build(),
		)
	}

	delete(cs.store.configuration, key)

	if cs.store.autoSaveEnabled {
		if err := cs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save configuration deletion").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (cs *jsonConfigurationStore) List(ctx context.Context) foundation.Result[map[string]any, error] {
	return cs.GetAll(ctx)
}

func (cs *jsonConfigurationStore) GetAll(ctx context.Context) foundation.Result[map[string]any, error] {
	cs.store.mu.RLock()
	defer cs.store.mu.RUnlock()

	// Return a deep copy to prevent external modification
	result := make(map[string]any, len(cs.store.configuration))
	for k, v := range cs.store.configuration {
		result[k] = v
	}

	return foundation.Ok[map[string]any, error](result)
}
