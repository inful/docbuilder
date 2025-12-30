package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// Scheduler wraps gocron scheduler for managing periodic tasks.
type Scheduler struct {
	scheduler gocron.Scheduler
	daemon    *Daemon // back-reference for injecting metadata into jobs
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

// SetDaemon injects a daemon reference post-construction to avoid an import cycle.
func (s *Scheduler) SetDaemon(d *Daemon) { s.daemon = d }

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

// SchedulePeriodicBuild schedules a periodic build job
// Returns the job ID for later management.
func (s *Scheduler) SchedulePeriodicBuild(interval time.Duration, jobType BuildType, repos []interface{}) (string, error) {
	job, err := s.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(s.executeBuild, jobType, repos),
		gocron.WithName(fmt.Sprintf("%s-build", jobType)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create periodic build job: %w", err)
	}

	return job.ID().String(), nil
}

// executeBuild is called by gocron to execute a scheduled build.
func (s *Scheduler) executeBuild(jobType BuildType, repos []interface{}) {
	if s.daemon == nil {
		slog.Error("Daemon reference not set in scheduler")
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
		TypedMeta: &BuildJobMetadata{
			V2Config:      s.daemon.config,
			StateManager:  s.daemon.stateManager,
			LiveReloadHub: s.daemon.liveReload,
		},
	}

	if err := s.daemon.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue scheduled build",
			logfields.JobID(jobID),
			logfields.Error(err))
	}
}
