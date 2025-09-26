package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// BuildType represents the type of build job
type BuildType string

const (
	BuildTypeManual    BuildType = "manual"    // Manually triggered build
	BuildTypeScheduled BuildType = "scheduled" // Cron-triggered build
	BuildTypeWebhook   BuildType = "webhook"   // Webhook-triggered build
	BuildTypeDiscovery BuildType = "discovery" // Auto-build after discovery
)

// BuildPriority represents the priority of a build job
type BuildPriority int

const (
	PriorityLow    BuildPriority = 1
	PriorityNormal BuildPriority = 2
	PriorityHigh   BuildPriority = 3
	PriorityUrgent BuildPriority = 4
)

// BuildStatus represents the current status of a build job
type BuildStatus string

const (
	BuildStatusQueued    BuildStatus = "queued"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusCompleted BuildStatus = "completed"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"
)

// BuildJob represents a single build job in the queue
type BuildJob struct {
	ID          string                 `json:"id"`
	Type        BuildType              `json:"type"`
	Priority    BuildPriority          `json:"priority"`
	Status      BuildStatus            `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`

	// Internal processing
	cancel context.CancelFunc `json:"-"`
}

// BuildQueue manages the queue of build jobs
type BuildQueue struct {
	jobs        chan *BuildJob
	workers     int
	maxSize     int
	mu          sync.RWMutex
	active      map[string]*BuildJob
	history     []*BuildJob
	historySize int
	stopChan    chan struct{}
	wg          sync.WaitGroup
	builder     Builder
	// retry policy (simple for now)
	maxRetries int
}

// NewBuildQueue creates a new build queue with the specified size and worker count
func NewBuildQueue(maxSize, workers int) *BuildQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	if workers <= 0 {
		workers = 2
	}

	return &BuildQueue{
		jobs:        make(chan *BuildJob, maxSize),
		workers:     workers,
		maxSize:     maxSize,
		active:      make(map[string]*BuildJob),
		history:     make([]*BuildJob, 0),
		historySize: 50, // Keep last 50 completed jobs
		stopChan:    make(chan struct{}),
		builder:     NewSiteBuilder(),
		maxRetries:  0,
	}
}

// Start begins processing jobs with the configured number of workers
func (bq *BuildQueue) Start(ctx context.Context) {
	slog.Info("Starting build queue", "workers", bq.workers, "max_size", bq.maxSize)

	for i := 0; i < bq.workers; i++ {
		bq.wg.Add(1)
		go bq.worker(ctx, fmt.Sprintf("worker-%d", i))
	}
}

// Stop gracefully shuts down the build queue
func (bq *BuildQueue) Stop(ctx context.Context) {
	slog.Info("Stopping build queue")

	close(bq.stopChan)

	// Cancel all active jobs
	bq.mu.Lock()
	for _, job := range bq.active {
		if job.cancel != nil {
			job.cancel()
		}
	}
	bq.mu.Unlock()

	bq.wg.Wait()
	slog.Info("Build queue stopped")
}

// Enqueue adds a new build job to the queue
func (bq *BuildQueue) Enqueue(job *BuildJob) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	if job.ID == "" {
		return fmt.Errorf("job ID is required")
	}

	job.Status = BuildStatusQueued

	select {
	case bq.jobs <- job:
		slog.Info("Build job enqueued", "job_id", job.ID, "type", job.Type, "priority", job.Priority)
		return nil
	default:
		return fmt.Errorf("build queue is full")
	}
}

// Length returns the current queue length
func (bq *BuildQueue) Length() int {
	return len(bq.jobs)
}

// GetActiveJobs returns a copy of currently active jobs
func (bq *BuildQueue) GetActiveJobs() []*BuildJob {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	active := make([]*BuildJob, 0, len(bq.active))
	for _, job := range bq.active {
		active = append(active, job)
	}
	return active
}

// GetHistory returns recent completed jobs
func (bq *BuildQueue) GetHistory() []*BuildJob {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	history := make([]*BuildJob, len(bq.history))
	copy(history, bq.history)
	return history
}

// worker processes jobs from the queue
func (bq *BuildQueue) worker(ctx context.Context, workerID string) {
	defer bq.wg.Done()

	slog.Debug("Build worker started", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Build worker stopped by context", "worker_id", workerID)
			return
		case <-bq.stopChan:
			slog.Debug("Build worker stopped by stop signal", "worker_id", workerID)
			return
		case job := <-bq.jobs:
			if job != nil {
				bq.processJob(ctx, job, workerID)
			}
		}
	}
}

// processJob handles the execution of a single build job
func (bq *BuildQueue) processJob(ctx context.Context, job *BuildJob, workerID string) {
	// Create job context with cancellation
	jobCtx, cancel := context.WithCancel(ctx)
	job.cancel = cancel
	defer cancel()

	// Mark job as running
	startTime := time.Now()
	job.StartedAt = &startTime
	job.Status = BuildStatusRunning

	bq.mu.Lock()
	bq.active[job.ID] = job
	bq.mu.Unlock()

	slog.Info("Build job started", "job_id", job.ID, "type", job.Type, "worker", workerID)

	// Execute the build
	err := bq.executeBuild(jobCtx, job)

	// Mark job as completed
	endTime := time.Now()
	job.CompletedAt = &endTime
	job.Duration = endTime.Sub(*job.StartedAt)

	bq.mu.Lock()
	delete(bq.active, job.ID)
	bq.addToHistory(job)
	bq.mu.Unlock()

	if err != nil {
		job.Status = BuildStatusFailed
		job.Error = err.Error()
		slog.Error("Build job failed",
			"job_id", job.ID,
			"type", job.Type,
			"duration", job.Duration,
			"error", err)
	} else {
		job.Status = BuildStatusCompleted
		slog.Info("Build job completed",
			"job_id", job.ID,
			"type", job.Type,
			"duration", job.Duration)
	}
}

// executeBuild performs the actual build process
func (bq *BuildQueue) executeBuild(ctx context.Context, job *BuildJob) error {
	// Route all build types through unified builder; specialization can evolve inside builder if needed.
	attempts := 0
	for {
		attempts++
		report, err := bq.builder.Build(ctx, job)
		if job.Metadata == nil {
			job.Metadata = make(map[string]interface{})
		}
		if report != nil {
			job.Metadata["build_report"] = report
		}
		if err == nil {
			return nil
		}
		// Determine if retry is allowed (placeholder: look for transient StageError in report)
		transient := false
		if report != nil && len(report.Errors) > 0 {
			for _, e := range report.Errors {
				if se, ok := e.(*hugo.StageError); ok && se.Transient() {
					transient = true
					break
				}
			}
		}
		if !transient || attempts > bq.maxRetries {
			if transient && attempts > bq.maxRetries {
				slog.Warn("Transient error but retries exhausted", "job_id", job.ID, "attempts", attempts)
			}
			return err
		}
		slog.Warn("Transient build error, retrying", "job_id", job.ID, "attempt", attempts, "max_retries", bq.maxRetries, "error", err)
		time.Sleep(time.Duration(attempts) * time.Second) // simple linear backoff
	}
}

// (Legacy per-type build wrapper methods removed; Builder abstraction handles all types.)

// addToHistory adds a completed job to the history, maintaining the size limit
func (bq *BuildQueue) addToHistory(job *BuildJob) {
	bq.history = append(bq.history, job)

	// Maintain history size limit
	if len(bq.history) > bq.historySize {
		// Remove oldest entries
		copy(bq.history, bq.history[len(bq.history)-bq.historySize:])
		bq.history = bq.history[:bq.historySize]
	}
}
