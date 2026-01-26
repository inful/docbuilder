package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// Scheduler wraps gocron scheduler for managing periodic tasks.
type Scheduler struct {
	scheduler gocron.Scheduler
	enqueuer  interface {
		Enqueue(job *BuildJob) error
	}
	metaFactory func() *BuildJobMetadata
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

// SetEnqueuer injects the queue/job enqueuer.
func (s *Scheduler) SetEnqueuer(e interface{ Enqueue(job *BuildJob) error }) { s.enqueuer = e }

// SetMetaFactory injects a factory for per-job metadata.
func (s *Scheduler) SetMetaFactory(f func() *BuildJobMetadata) { s.metaFactory = f }

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

// SchedulePeriodicBuild schedules a periodic build job
// Returns the job ID for later management.
func (s *Scheduler) SchedulePeriodicBuild(interval time.Duration, jobType BuildType, repos []any) (string, error) {
	job, err := s.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(s.executeBuild, jobType, repos),
		gocron.WithName(fmt.Sprintf("%s-build", jobType)),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create periodic build job: %w", err)
	}

	return job.ID().String(), nil
}

// executeBuild is called by gocron to execute a scheduled build.
func (s *Scheduler) executeBuild(jobType BuildType, repos []any) {
	if s.enqueuer == nil {
		slog.Error("Scheduler enqueuer not set")
		return
	}
	if s.metaFactory == nil {
		slog.Error("Scheduler metadata factory not set")
		return
	}

	jobID := fmt.Sprintf("%s-%d", jobType, time.Now().Unix())
	slog.Info("Executing scheduled build",
		logfields.JobID(jobID),
		slog.String("type", string(jobType)))

	job := &BuildJob{
		ID:        jobID,
		Type:      jobType,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
		TypedMeta: s.metaFactory(),
	}

	if err := s.enqueuer.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue scheduled build",
			logfields.JobID(jobID),
			logfields.Error(err))
	}
}
