package state

import (
	"context"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonScheduleStore implements ScheduleStore for the JSON store.
type jsonScheduleStore struct {
	store *JSONStore
}

func (ss *jsonScheduleStore) Create(_ context.Context, schedule *Schedule) foundation.Result[*Schedule, error] {
	if schedule == nil {
		return foundation.Err[*Schedule, error](
			foundation.ValidationError("schedule cannot be nil").Build(),
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
				foundation.InternalError("failed to save schedule").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Schedule, error](schedule)
}

func (ss *jsonScheduleStore) GetByID(_ context.Context, id string) foundation.Result[foundation.Option[*Schedule], error] {
	if id == "" {
		return foundation.Err[foundation.Option[*Schedule], error](
			foundation.ValidationError("ID cannot be empty").Build(),
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
			foundation.ValidationError("schedule cannot be nil").Build(),
		)
	}

	if validationResult := schedule.Validate(); !validationResult.Valid {
		return foundation.Err[*Schedule, error](validationResult.ToError())
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	if _, exists := ss.store.schedules[schedule.ID]; !exists {
		return foundation.Err[*Schedule, error](
			foundation.NotFoundError("schedule").
				WithContext(foundation.Fields{"id": schedule.ID}).
				Build(),
		)
	}

	schedule.UpdatedAt = time.Now()
	ss.store.schedules[schedule.ID] = schedule

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Schedule, error](
				foundation.InternalError("failed to save schedule update").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Schedule, error](schedule)
}

func (ss *jsonScheduleStore) Delete(_ context.Context, id string) foundation.Result[struct{}, error] {
	if id == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("ID cannot be empty").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	if _, exists := ss.store.schedules[id]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("schedule").
				WithContext(foundation.Fields{"id": id}).
				Build(),
		)
	}

	delete(ss.store.schedules, id)

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save schedule deletion").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
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

func (ss *jsonScheduleStore) GetActive(_ context.Context) foundation.Result[[]Schedule, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	schedules := make([]Schedule, 0)
	for _, schedule := range ss.store.schedules {
		// Check if schedule is active
		if schedule.IsActive {
			schedules = append(schedules, *schedule)
		}
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
