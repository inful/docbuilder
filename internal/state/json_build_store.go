package state

import (
	"context"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonBuildStore implements BuildStore for the JSON store.
type jsonBuildStore struct {
	store *JSONStore
}

func (bs *jsonBuildStore) Create(ctx context.Context, build *Build) foundation.Result[*Build, error] {
	if build == nil {
		return foundation.Err[*Build, error](
			foundation.ValidationError("build cannot be nil").Build(),
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
				foundation.InternalError("failed to save build").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Build, error](build)
}

func (bs *jsonBuildStore) GetByID(ctx context.Context, id string) foundation.Result[foundation.Option[*Build], error] {
	if id == "" {
		return foundation.Err[foundation.Option[*Build], error](
			foundation.ValidationError("ID cannot be empty").Build(),
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

func (bs *jsonBuildStore) Update(ctx context.Context, build *Build) foundation.Result[*Build, error] {
	if build == nil {
		return foundation.Err[*Build, error](
			foundation.ValidationError("build cannot be nil").Build(),
		)
	}

	if validationResult := build.Validate(); !validationResult.Valid {
		return foundation.Err[*Build, error](validationResult.ToError())
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	if _, exists := bs.store.builds[build.ID]; !exists {
		return foundation.Err[*Build, error](
			foundation.NotFoundError("build").
				WithContext(foundation.Fields{"id": build.ID}).
				Build(),
		)
	}

	build.UpdatedAt = time.Now()
	bs.store.builds[build.ID] = build

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Build, error](
				foundation.InternalError("failed to save build update").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Build, error](build)
}

func (bs *jsonBuildStore) List(ctx context.Context, opts ListOptions) foundation.Result[[]Build, error] {
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

func (bs *jsonBuildStore) Delete(ctx context.Context, id string) foundation.Result[struct{}, error] {
	if id == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("ID cannot be empty").Build(),
		)
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	if _, exists := bs.store.builds[id]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("build").
				WithContext(foundation.Fields{"id": id}).
				Build(),
		)
	}

	delete(bs.store.builds, id)

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save build deletion").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (bs *jsonBuildStore) Cleanup(ctx context.Context, maxBuilds int) foundation.Result[int, error] {
	if maxBuilds <= 0 {
		return foundation.Err[int, error](
			foundation.ValidationError("maxBuilds must be positive").Build(),
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
				foundation.InternalError("failed to save build cleanup").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[int, error](deletedCount)
}
