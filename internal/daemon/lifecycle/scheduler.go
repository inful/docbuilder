// Package lifecycle owns the daemon's lifecycle primitives: the cron-style
// Scheduler, the WorkerGroup that tracks background goroutines, and the
// StopAwareContext helper that lets long-running work be canceled when the
// daemon's stop channel closes.
//
// The package has no dependency on the rest of the daemon package. The daemon
// owns a single Scheduler and WorkerGroup instance and calls them through
// type aliases defined in internal/daemon/lifecycle_aliases.go.
package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// Scheduler wraps gocron scheduler for managing periodic tasks.
type Scheduler struct {
	scheduler gocron.Scheduler
}

// NewScheduler creates a new scheduler instance.
func NewScheduler() (*Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, derrors.WrapError(err, derrors.CategoryInternal, "failed to create gocron scheduler").Build()
	}

	return &Scheduler{
		scheduler: s,
	}, nil
}

// Start begins the scheduler.
func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("Starting scheduler")
	s.scheduler.Start()
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop(ctx context.Context) error {
	slog.Info("Stopping scheduler")
	return s.scheduler.Shutdown()
}

// ScheduleEvery schedules a duration-based job.
//
// The job runs in singleton mode to avoid overlapping executions.
func (s *Scheduler) ScheduleEvery(name string, interval time.Duration, task func()) (string, error) {
	if interval <= 0 {
		return "", errors.New("interval must be greater than zero")
	}

	job, err := s.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(task),
		gocron.WithName(name),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return "", derrors.WrapError(err, derrors.CategoryInternal, "failed to create duration job").Build()
	}

	return job.ID().String(), nil
}

// ScheduleCron schedules a cron-based job.
//
// The job runs in singleton mode to avoid overlapping executions.
func (s *Scheduler) ScheduleCron(name, expression string, task func()) (string, error) {
	job, err := s.scheduler.NewJob(
		gocron.CronJob(expression, false),
		gocron.NewTask(task),
		gocron.WithName(name),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return "", derrors.WrapError(err, derrors.CategoryInternal, "failed to create cron job").Build()
	}

	return job.ID().String(), nil
}
