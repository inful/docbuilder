package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
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
	// TODO: Implement actual build logic
	// For now, simulate build work

	switch job.Type {
	case BuildTypeManual:
		return bq.executeManualBuild(ctx, job)
	case BuildTypeScheduled:
		return bq.executeScheduledBuild(ctx, job)
	case BuildTypeWebhook:
		return bq.executeWebhookBuild(ctx, job)
	case BuildTypeDiscovery:
		return bq.executeDiscoveryBuild(ctx, job)
	default:
		return fmt.Errorf("unsupported build type: %s", job.Type)
	}
}

// executeManualBuild handles manually triggered builds
func (bq *BuildQueue) executeManualBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing manual build", "job_id", job.ID)

	// Simulate build work
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		// TODO: Implement actual Hugo site generation
		return nil
	}
}

// executeScheduledBuild handles cron-triggered builds
func (bq *BuildQueue) executeScheduledBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing scheduled build", "job_id", job.ID)

	// Simulate build work
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		// TODO: Implement discovery + build flow
		return nil
	}
}

// executeWebhookBuild handles webhook-triggered builds
func (bq *BuildQueue) executeWebhookBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing webhook build", "job_id", job.ID)

	// Extract webhook information from metadata
	if repo, ok := job.Metadata["repository"].(string); ok {
		slog.Info("Building for repository", "repository", repo)
	}

	// Simulate build work
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
		// TODO: Implement targeted repository build
		return nil
	}
}

// executeDiscoveryBuild handles auto-builds after discovery
func (bq *BuildQueue) executeDiscoveryBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing discovery build", "job_id", job.ID)

	// Extract discovery result from metadata
	if result, ok := job.Metadata["discovery_result"]; ok {
		slog.Info("Building with discovery result", "type", fmt.Sprintf("%T", result))
	}

	// Simulate build work
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(7 * time.Second):
		// TODO: Implement full site rebuild with discovered repositories
		return nil
	}
}

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
