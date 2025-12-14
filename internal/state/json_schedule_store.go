package state

import (
	"context"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonScheduleStore implements ScheduleStore for the JSON store.
type jsonScheduleStore struct {
	store *JSONStore
}

func (ss *jsonScheduleStore) Create(_ context.Context, schedule *Schedule) foundation.Result[*Schedule, error] {
	if schedule == nil {
		return foundation.Err[*Schedule, error](
			errors.ValidationError("schedule cannot be nil").Build(),
		)
	}

	if validationResult := schedule.Validate(); !validationResult.Valid {
		return foundation.Err[*Schedule, error](validationResult.ToError())
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	ss.store.schedules[schedule.ID] = schedule

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			delete(ss.store.schedules, schedule.ID)
			return foundation.Err[*Schedule, error](
				errors.InternalError("failed to save schedule").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Schedule, error](schedule)
}

func (ss *jsonScheduleStore) GetByID(_ context.Context, id string) foundation.Result[foundation.Option[*Schedule], error] {
	if id == "" {
		return foundation.Err[foundation.Option[*Schedule], error](
			errors.ValidationError("ID cannot be empty").Build(),
		)
	}

	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	if schedule, exists := ss.store.schedules[id]; exists {
		scheduleCopy := *schedule
		return foundation.Ok[foundation.Option[*Schedule], error](foundation.Some(&scheduleCopy))
	}

	return foundation.Ok[foundation.Option[*Schedule], error](foundation.None[*Schedule]())
}

func (ss *jsonScheduleStore) Update(_ context.Context, schedule *Schedule) foundation.Result[*Schedule, error] {
	if schedule == nil {
		return foundation.Err[*Schedule, error](
			errors.ValidationError("schedule cannot be nil").Build(),
		)
	}

	if validationResult := schedule.Validate(); !validationResult.Valid {
		return foundation.Err[*Schedule, error](validationResult.ToError())
	}

	return updateEntity[Schedule](
		ss.store,
		schedule,
		func() bool { _, ok := ss.store.schedules[schedule.ID]; return ok },
		func() { schedule.UpdatedAt = time.Now() },
		func() { ss.store.schedules[schedule.ID] = schedule },
		func() foundation.Result[*Schedule, error] {
			return foundation.Err[*Schedule, error](
				errors.NotFoundError("schedule").
					WithContext("id", schedule.ID).
					Build(),
			)
		},
		"failed to save schedule update",
	)
}

func (ss *jsonScheduleStore) Delete(_ context.Context, id string) foundation.Result[struct{}, error] {
	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	return deleteEntity(
		id,
		func() bool { _, exists := ss.store.schedules[id]; return exists },
		func() { delete(ss.store.schedules, id) },
		func() error {
			if ss.store.autoSaveEnabled {
				return ss.store.saveToDiskUnsafe()
			}
			return nil
		},
		"schedule",
		"failed to save schedule deletion",
	)
}

func (ss *jsonScheduleStore) List(_ context.Context) foundation.Result[[]Schedule, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	schedules := make([]Schedule, 0, len(ss.store.schedules))
	for _, schedule := range ss.store.schedules {
		schedules = append(schedules, *schedule)
	}

	// Sort by next run time
	sort.Slice(schedules, func(i, j int) bool {
		// Handle Option[time.Time] properly
		if !schedules[i].NextRun.IsSome() {
			return false
		}
		if !schedules[j].NextRun.IsSome() {
			return true
		}
		return schedules[i].NextRun.Unwrap().Before(schedules[j].NextRun.Unwrap())
	})

	return foundation.Ok[[]Schedule, error](schedules)
}
