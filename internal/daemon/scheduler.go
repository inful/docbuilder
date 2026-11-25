package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// Scheduler wraps gocron scheduler for managing periodic tasks.
type Scheduler struct {
	mu        sync.Mutex
	scheduler gocron.Scheduler
	stopped   bool
}

// NewScheduler creates a new scheduler instance.
func NewScheduler() (*Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("failed to create gocron scheduler: %w", err)
	}

	return &Scheduler{
		scheduler: s,
	}, nil
}

// Reset gives the scheduler a new engine.
//
// The engine of gocron cannot be started again after it was shut down:
// the periodic work of the daemon stops for good after a restart of the
// daemon. The jobs are scheduled again after the reset.
func (s *Scheduler) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to create gocron scheduler: %w", err)
	}
	s.scheduler = scheduler
	s.stopped = false
	return nil
}

// Start begins the scheduler.
//
// The scheduler holds no job of its own: the jobs carry the context of
// the run, and they stop with it.
func (s *Scheduler) Start(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.Info("Starting scheduler")
	if s.scheduler == nil {
		return
	}
	s.scheduler.Start()
}

// Stop gracefully shuts down the scheduler.
//
// The deadline of the context is the deadline of the stop: a job that
// runs longer than that does not hold the daemon.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.Info("Stopping scheduler")
	if s.scheduler == nil {
		return nil
	}
	if s.stopped {
		return nil
	}
	s.stopped = true
	if ctx == nil {
		return s.scheduler.Shutdown()
	}
	return s.scheduler.ShutdownWithContext(ctx)
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
		return "", fmt.Errorf("failed to create duration job: %w", err)
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
		return "", fmt.Errorf("failed to create cron job: %w", err)
	}

	return job.ID().String(), nil
}
