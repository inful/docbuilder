package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/retry"
)

// BuildType represents the type of build job.
type BuildType string

const (
	BuildTypeManual    BuildType = "manual"    // Manually triggered build
	BuildTypeScheduled BuildType = "scheduled" // Cron-triggered build
	BuildTypeWebhook   BuildType = "webhook"   // Webhook-triggered build
	BuildTypeDiscovery BuildType = "discovery" // Auto-build after discovery
)

// BuildPriority represents the priority of a build job.
type BuildPriority int

const (
	PriorityLow    BuildPriority = 1
	PriorityNormal BuildPriority = 2
	PriorityHigh   BuildPriority = 3
	PriorityUrgent BuildPriority = 4
)

// BuildStatus represents the current status of a build job.
type BuildStatus string

const (
	BuildStatusQueued    BuildStatus = "queued"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusCompleted BuildStatus = "completed"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "canceled"
)

// BuildJob represents a single build job in the queue.
type BuildJob struct {
	ID          string        `json:"id"`
	Type        BuildType     `json:"type"`
	Priority    BuildPriority `json:"priority"`
	Status      BuildStatus   `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Error       string        `json:"error,omitempty"`

	// TypedMeta holds typed metadata for the build job.
	// This provides compile-time safety for job configuration and dependencies.
	TypedMeta *BuildJobMetadata `json:"typed_meta,omitempty"`

	// Internal processing
	cancel context.CancelFunc `json:"-"`
}

// BuildEventEmitter abstracts event emission for build lifecycle events.
// This allows the BuildQueue to emit events without depending on the Daemon directly.
type BuildEventEmitter interface {
	EmitBuildStarted(ctx context.Context, buildID string, meta eventstore.BuildStartedMeta) error
	EmitBuildCompleted(ctx context.Context, buildID string, duration time.Duration, artifacts map[string]string) error
	EmitBuildFailed(ctx context.Context, buildID, stage, errorMsg string) error
	EmitBuildReport(ctx context.Context, buildID string, report *hugo.BuildReport) error
}

// BuildQueue manages the queue of build jobs.
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
	// retry policy configuration (source) + derived policy
	retryPolicy retry.Policy
	recorder    metrics.Recorder
	// Event emitter for build lifecycle events (Phase B)
	eventEmitter BuildEventEmitter
}

// NewBuildQueue creates a new build queue with the specified size, worker count, and builder.
// The builder parameter is required - use build.NewBuildService() wrapped in NewBuildServiceAdapter().
func NewBuildQueue(maxSize, workers int, builder Builder) *BuildQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	if workers <= 0 {
		workers = 2
	}
	if builder == nil {
		panic("NewBuildQueue: builder is required")
	}

	return &BuildQueue{
		jobs:        make(chan *BuildJob, maxSize),
		workers:     workers,
		maxSize:     maxSize,
		active:      make(map[string]*BuildJob),
		history:     make([]*BuildJob, 0),
		historySize: 50, // Keep last 50 completed jobs
		stopChan:    make(chan struct{}),
		builder:     builder,
		retryPolicy: retry.DefaultPolicy(),
		recorder:    metrics.NoopRecorder{},
	}
}

// ConfigureRetry updates the retry policy (should be called once at daemon init after config load).
func (bq *BuildQueue) ConfigureRetry(cfg config.BuildConfig) {
	retryInitialDelay, _ := time.ParseDuration(cfg.RetryInitialDelay)
	maxDelay, _ := time.ParseDuration(cfg.RetryMaxDelay)
	bq.retryPolicy = retry.NewPolicy(cfg.RetryBackoff, retryInitialDelay, maxDelay, cfg.MaxRetries)
}

// SetRecorder injects a metrics recorder for retry metrics (optional).
func (bq *BuildQueue) SetRecorder(r metrics.Recorder) {
	if r == nil {
		r = metrics.NoopRecorder{}
	}
	bq.recorder = r
}

// SetEventEmitter injects a build event emitter for Phase B event sourcing.
func (bq *BuildQueue) SetEventEmitter(emitter BuildEventEmitter) {
	bq.eventEmitter = emitter
}

// Start begins processing jobs with the configured number of workers.
func (bq *BuildQueue) Start(ctx context.Context) {
	slog.Info("Starting build queue", "workers", bq.workers, "max_size", bq.maxSize)

	for i := range bq.workers {
		bq.wg.Add(1)
		go bq.worker(ctx, fmt.Sprintf("worker-%d", i))
	}
}

// Stop gracefully shuts down the build queue.
func (bq *BuildQueue) Stop(_ context.Context) {
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

// Enqueue adds a new build job to the queue.
func (bq *BuildQueue) Enqueue(job *BuildJob) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}

	if job.ID == "" {
		return errors.New("job ID is required")
	}

	job.Status = BuildStatusQueued

	select {
	case bq.jobs <- job:
		slog.Info("Build job enqueued", logfields.JobID(job.ID), logfields.JobType(string(job.Type)), logfields.JobPriority(int(job.Priority)))
		return nil
	default:
		return errors.New("build queue is full")
	}
}

// Length returns the current queue length.
func (bq *BuildQueue) Length() int {
	return len(bq.jobs)
}

// GetActiveJobs returns a copy of currently active jobs.
func (bq *BuildQueue) GetActiveJobs() []*BuildJob {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	active := make([]*BuildJob, 0, len(bq.active))
	for _, job := range bq.active {
		active = append(active, job)
	}
	return active
}

// worker processes jobs from the queue.
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

// processJob handles the execution of a single build job.
func (bq *BuildQueue) processJob(ctx context.Context, job *BuildJob, workerID string) {
	// Create job context with cancellation
	jobCtx, cancel := context.WithCancel(ctx)
	job.cancel = cancel
	defer cancel()

	// Mark job as running and emit started event
	bq.markJobRunning(job, workerID)
	bq.emitBuildStartedEvent(jobCtx, job, workerID)

	// Execute the build
	err := bq.executeBuild(jobCtx, job)

	// Mark job as completed
	duration := bq.markJobCompleted(job, err)

	// Emit completion events
	bq.emitCompletionEvents(ctx, job, err, duration)

	// Log final status
	bq.logJobCompletion(job, err, duration)
}

// markJobRunning marks a job as running and activates it.
func (bq *BuildQueue) markJobRunning(job *BuildJob, workerID string) {
	startTime := time.Now()
	bq.mu.Lock()
	job.StartedAt = &startTime
	job.Status = BuildStatusRunning
	bq.active[job.ID] = job
	bq.mu.Unlock()

	slog.Info("Build job started", logfields.JobID(job.ID), logfields.JobType(string(job.Type)), logfields.Worker(workerID))
}

// emitBuildStartedEvent emits the build started event.
func (bq *BuildQueue) emitBuildStartedEvent(ctx context.Context, job *BuildJob, workerID string) {
	if bq.eventEmitter == nil {
		return
	}

	meta := eventstore.BuildStartedMeta{
		Type:     string(job.Type),
		Priority: int(job.Priority),
		WorkerID: workerID,
	}
	if err := bq.eventEmitter.EmitBuildStarted(ctx, job.ID, meta); err != nil {
		slog.Warn("Failed to emit BuildStarted event", logfields.JobID(job.ID), logfields.Error(err))
	}
}

// markJobCompleted marks a job as completed or failed and returns the duration.
func (bq *BuildQueue) markJobCompleted(job *BuildJob, err error) time.Duration {
	endTime := time.Now()
	bq.mu.Lock()
	job.CompletedAt = &endTime
	if job.StartedAt != nil {
		job.Duration = endTime.Sub(*job.StartedAt)
	}
	delete(bq.active, job.ID)
	bq.addToHistory(job)
	if err != nil {
		job.Status = BuildStatusFailed
		job.Error = err.Error()
	} else {
		job.Status = BuildStatusCompleted
	}
	duration := job.Duration
	bq.mu.Unlock()

	slog.Debug("Build job completed",
		logfields.JobID(job.ID),
		"has_error", err != nil,
		"event_emitter_nil", bq.eventEmitter == nil)

	return duration
}

// emitCompletionEvents emits build completion, failure, and report events.
func (bq *BuildQueue) emitCompletionEvents(ctx context.Context, job *BuildJob, err error, duration time.Duration) {
	if bq.eventEmitter == nil {
		return
	}

	// Always emit build report if available (for both success and failure)
	report := bq.extractBuildReport(job)
	bq.emitBuildReportEvent(ctx, job, report)

	// Emit success or failure event
	if err != nil {
		bq.emitBuildFailedEvent(ctx, job, err)
	} else {
		bq.emitBuildCompletedEvent(ctx, job, duration, report)
	}
}

// extractBuildReport extracts the build report from job metadata.
func (bq *BuildQueue) extractBuildReport(job *BuildJob) *hugo.BuildReport {
	if job.TypedMeta != nil && job.TypedMeta.BuildReport != nil {
		return job.TypedMeta.BuildReport
	}
	return nil
}

// emitBuildReportEvent emits the build report event if report is available.
func (bq *BuildQueue) emitBuildReportEvent(ctx context.Context, job *BuildJob, report *hugo.BuildReport) {
	slog.Debug("Build queue event emit check",
		logfields.JobID(job.ID),
		"emitter_nil", bq.eventEmitter == nil,
		"typed_meta_nil", job.TypedMeta == nil,
		"build_report_nil", report == nil)

	if report != nil {
		if err := bq.eventEmitter.EmitBuildReport(ctx, job.ID, report); err != nil {
			slog.Warn("Failed to emit BuildReport event", logfields.JobID(job.ID), logfields.Error(err))
		}
	} else {
		slog.Debug("Skipping EmitBuildReport - report is nil", logfields.JobID(job.ID))
	}
}

// emitBuildFailedEvent emits the build failed event.
func (bq *BuildQueue) emitBuildFailedEvent(ctx context.Context, job *BuildJob, err error) {
	if emitErr := bq.eventEmitter.EmitBuildFailed(ctx, job.ID, "build", err.Error()); emitErr != nil {
		slog.Warn("Failed to emit BuildFailed event", logfields.JobID(job.ID), logfields.Error(emitErr))
	}
}

// emitBuildCompletedEvent emits the build completed event with artifacts.
func (bq *BuildQueue) emitBuildCompletedEvent(ctx context.Context, job *BuildJob, duration time.Duration, report *hugo.BuildReport) {
	artifacts := make(map[string]string)
	// Extract artifacts from build report if available
	if report != nil {
		artifacts["files"] = strconv.Itoa(report.Files)
		artifacts["repositories"] = strconv.Itoa(report.Repositories)
	}
	if err := bq.eventEmitter.EmitBuildCompleted(ctx, job.ID, duration, artifacts); err != nil {
		slog.Warn("Failed to emit BuildCompleted event", logfields.JobID(job.ID), logfields.Error(err))
	}
}

// logJobCompletion logs the final job completion status.
func (bq *BuildQueue) logJobCompletion(job *BuildJob, err error, duration time.Duration) {
	if err != nil {
		slog.Error("Build job failed", logfields.JobID(job.ID), logfields.JobType(string(job.Type)), slog.Duration("duration", duration), logfields.Error(err))
	} else {
		slog.Info("Build job completed", logfields.JobID(job.ID), logfields.JobType(string(job.Type)), slog.Duration("duration", duration))
	}
}

// executeBuild performs the actual build process.
func (bq *BuildQueue) executeBuild(ctx context.Context, job *BuildJob) error {
	// Route all build types through unified builder using retryPolicy.
	attempts := 0
	policy := bq.retryPolicy
	if policy.Initial <= 0 {
		policy = retry.DefaultPolicy()
	} // fallback safety
	totalRetries := 0
	exhausted := false

	for {
		attempts++
		report, err := bq.builder.Build(ctx, job)
		// Store report in TypedMeta
		if report != nil {
			meta := EnsureTypedMeta(job)
			meta.BuildReport = report
		}
		if err == nil {
			// attach retry summary if present
			if report != nil && totalRetries > 0 {
				report.Retries = totalRetries
				report.RetriesExhausted = exhausted
			}
			return nil
		}
		// Determine if retry is allowed (look for transient StageError in report)
		transient, transientStage := findTransientError(report)
		
		if shouldStopRetrying(transient, totalRetries, policy.MaxRetries) {
			handleRetriesExhausted(job, report, transient, totalRetries, transientStage, bq.recorder)
			return err
		}
		// perform retry
		totalRetries++
		rec := extractRecorder(report, bq.recorder)
		if rec != nil && transientStage != "" {
			rec.IncBuildRetry(transientStage)
		}
		delay := policy.Delay(totalRetries)
		slog.Warn("Transient build error, retrying", logfields.JobID(job.ID), slog.Int("attempt", attempts), slog.Int("retry", totalRetries), slog.Int("max_retries", policy.MaxRetries), logfields.Stage(transientStage), slog.Duration("delay", delay), logfields.Error(err))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// findTransientError checks if report contains a transient error.
func findTransientError(report *hugo.BuildReport) (bool, string) {
	if report == nil || len(report.Errors) == 0 {
		return false, ""
	}
	
	for _, e := range report.Errors {
		var se *hugo.StageError
		if errors.As(e, &se) && se.Transient() {
			return true, string(se.Stage)
		}
	}
	return false, ""
}

// shouldStopRetrying determines if retrying should stop.
func shouldStopRetrying(transient bool, totalRetries, maxRetries int) bool {
	return !transient || totalRetries >= maxRetries
}

// handleRetriesExhausted logs and records exhausted retry attempts.
func handleRetriesExhausted(job *BuildJob, report *hugo.BuildReport, transient bool, totalRetries int, transientStage string, recorder metrics.Recorder) {
	if !transient || totalRetries < 1 {
		return
	}
	
	slog.Warn("Transient error but retries exhausted", logfields.JobID(job.ID), slog.Int("total_retries", totalRetries))
	
	if report != nil {
		report.Retries = totalRetries
		report.RetriesExhausted = true
	}
	
	rec := extractRecorder(report, recorder)
	if rec != nil && transientStage != "" {
		rec.IncBuildRetryExhausted(transientStage)
	}
}

// extractRecorder fetches Recorder from embedded report's generator if available via type assertion on metadata (best effort).
func extractRecorder(_ *hugo.BuildReport, fallback metrics.Recorder) metrics.Recorder {
	// Currently we only have fallback; future: attempt to derive from report metadata if embedded.
	return fallback
}

// (Legacy per-type build wrapper methods removed; Builder abstraction handles all types.)

// addToHistory adds a completed job to the history, maintaining the size limit.
func (bq *BuildQueue) addToHistory(job *BuildJob) {
	bq.history = append(bq.history, job)

	// Maintain history size limit
	if len(bq.history) > bq.historySize {
		// Remove oldest entries
		copy(bq.history, bq.history[len(bq.history)-bq.historySize:])
		bq.history = bq.history[:bq.historySize]
	}
}

// JobSnapshot returns a copy of the job (searching active then history) under lock for race-free observation in tests/handlers.
func (bq *BuildQueue) JobSnapshot(id string) (*BuildJob, bool) {
	bq.mu.RLock()
	defer bq.mu.RUnlock()
	if j, ok := bq.active[id]; ok {
		cp := *j
		return &cp, true
	}
	for _, j := range bq.history {
		if j.ID == id {
			cp := *j
			return &cp, true
		}
	}
	return nil, false
}
