package state

import (
	"context"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// jsonBuildStore implements BuildStore for the JSON store.
type jsonBuildStore struct {
	store *JSONStore
}

func (bs *jsonBuildStore) Create(_ context.Context, build *Build) foundation.Result[*Build, error] {
	if build == nil {
		return foundation.Err[*Build, error](
			errors.ValidationError("build cannot be nil").Build(),
		)
	}

	if validationResult := build.Validate(); !validationResult.Valid {
		return foundation.Err[*Build, error](validationResult.ToError())
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	now := time.Now()
	build.CreatedAt = now
	build.UpdatedAt = now

	bs.store.builds[build.ID] = build

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			delete(bs.store.builds, build.ID)
			return foundation.Err[*Build, error](
				errors.InternalError("failed to save build").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Build, error](build)
}

func (bs *jsonBuildStore) GetByID(_ context.Context, id string) foundation.Result[foundation.Option[*Build], error] {
	if id == "" {
		return foundation.Err[foundation.Option[*Build], error](
			errors.ValidationError("ID cannot be empty").Build(),
		)
	}

	bs.store.mu.RLock()
	defer bs.store.mu.RUnlock()

	if build, exists := bs.store.builds[id]; exists {
		buildCopy := *build
		return foundation.Ok[foundation.Option[*Build], error](foundation.Some(&buildCopy))
	}

	return foundation.Ok[foundation.Option[*Build], error](foundation.None[*Build]())
}

func (bs *jsonBuildStore) Update(_ context.Context, build *Build) foundation.Result[*Build, error] {
	if build == nil {
		return foundation.Err[*Build, error](
			errors.ValidationError("build cannot be nil").Build(),
		)
	}

	if validationResult := build.Validate(); !validationResult.Valid {
		return foundation.Err[*Build, error](validationResult.ToError())
	}

	return updateEntity[Build](
		bs.store,
		build,
		func() bool { _, ok := bs.store.builds[build.ID]; return ok },
		func() { build.UpdatedAt = time.Now() },
		func() { bs.store.builds[build.ID] = build },
		func() foundation.Result[*Build, error] {
			return foundation.Err[*Build, error](
				errors.NotFoundError("build").
					WithContext("id", build.ID).
					Build(),
			)
		},
		"failed to save build update",
	)
}

func (bs *jsonBuildStore) List(_ context.Context, opts ListOptions) foundation.Result[[]Build, error] {
	bs.store.mu.RLock()
	defer bs.store.mu.RUnlock()

	builds := make([]Build, 0, len(bs.store.builds))
	for _, build := range bs.store.builds {
		builds = append(builds, *build)
	}

	// Sort by creation time (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].CreatedAt.After(builds[j].CreatedAt)
	})

	// Apply pagination if specified
	if opts.Limit.IsSome() && opts.Limit.Unwrap() > 0 {
		start := 0
		if opts.Offset.IsSome() {
			start = opts.Offset.Unwrap()
		}

		if start > len(builds) {
			start = len(builds)
		}

		end := start + opts.Limit.Unwrap()
		if end > len(builds) {
			end = len(builds)
		}

		builds = builds[start:end]
	}

	return foundation.Ok[[]Build, error](builds)
}

func (bs *jsonBuildStore) Delete(_ context.Context, id string) foundation.Result[struct{}, error] {
	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	return deleteEntity(
		id,
		func() bool { _, exists := bs.store.builds[id]; return exists },
		func() { delete(bs.store.builds, id) },
		func() error {
			if bs.store.autoSaveEnabled {
				return bs.store.saveToDiskUnsafe()
			}
			return nil
		},
		"build",
		"failed to save build deletion",
	)
}

func (bs *jsonBuildStore) Cleanup(_ context.Context, maxBuilds int) foundation.Result[int, error] {
	if maxBuilds <= 0 {
		return foundation.Err[int, error](
			errors.ValidationError("maxBuilds must be positive").Build(),
		)
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	builds := make([]*Build, 0, len(bs.store.builds))
	for _, build := range bs.store.builds {
		builds = append(builds, build)
	}

	if len(builds) <= maxBuilds {
		return foundation.Ok[int, error](0) // No cleanup needed
	}

	// Sort by creation time (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].CreatedAt.After(builds[j].CreatedAt)
	})

	// Keep only the newest maxBuilds
	toDelete := builds[maxBuilds:]
	deletedCount := len(toDelete)

	for _, build := range toDelete {
		delete(bs.store.builds, build.ID)
	}

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[int, error](
				errors.InternalError("failed to save build cleanup").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[int, error](deletedCount)
}
