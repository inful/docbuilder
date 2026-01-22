package queue

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
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

	TypedMeta *BuildJobMetadata `json:"typed_meta,omitempty"`

	// Internal processing
	cancel context.CancelFunc `json:"-"`
}

// Builder executes a build job and returns a build report.
type Builder interface {
	Build(ctx context.Context, job *BuildJob) (*models.BuildReport, error)
}

// BuildEventEmitter abstracts event emission for build lifecycle events.
// This allows the BuildQueue to emit events without depending on a daemon implementation.
type BuildEventEmitter interface {
	EmitBuildStarted(ctx context.Context, buildID string, meta eventstore.BuildStartedMeta) error
	EmitBuildCompleted(ctx context.Context, buildID string, duration time.Duration, artifacts map[string]string) error
	EmitBuildFailed(ctx context.Context, buildID, stage, errorMsg string) error
	EmitBuildReport(ctx context.Context, buildID string, report *models.BuildReport) error
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

	retryPolicy retry.Policy
	recorder    metrics.Recorder

	eventEmitter BuildEventEmitter
}

// New creates a new build queue.
func New(maxSize, workers int, builder Builder) *BuildQueue {
	return NewBuildQueue(maxSize, workers, builder)
}

// NewBuildQueue creates a new build queue with the specified size, worker count, and builder.
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
		historySize: 50,
		stopChan:    make(chan struct{}),
		builder:     builder,
		retryPolicy: retry.DefaultPolicy(),
		recorder:    metrics.NoopRecorder{},
	}
}

// ConfigureRetry updates the retry policy (should be called once after config load).
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

// SetEventEmitter injects a build event emitter.
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
}

// Length returns the current queue length.
func (bq *BuildQueue) Length() int {
	return len(bq.jobs)
}

// GetActiveJobs returns a copy of the currently active jobs.
func (bq *BuildQueue) GetActiveJobs() []*BuildJob {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	active := make([]*BuildJob, 0, len(bq.active))
	for _, job := range bq.active {
		active = append(active, job)
	}
	return active
}

// Enqueue adds a new build job to the queue.
func (bq *BuildQueue) Enqueue(job *BuildJob) error {
	if job == nil {
		return stdErrors.New("job cannot be nil")
	}
	if job.ID == "" {
		return stdErrors.New("job ID is required")
	}

	job.Status = BuildStatusQueued

	select {
	case bq.jobs <- job:
		return nil
	default:
		return stdErrors.New("build queue is full")
	}
}

// JobSnapshot returns a copy of a job (active first, then history).
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

func (bq *BuildQueue) worker(ctx context.Context, workerID string) {
	defer bq.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-bq.stopChan:
			return
		case job := <-bq.jobs:
			if job != nil {
				bq.processJob(ctx, job, workerID)
			}
		}
	}
}

func (bq *BuildQueue) processJob(ctx context.Context, job *BuildJob, workerID string) {
	jobCtx, cancel := context.WithCancel(ctx)
	job.cancel = cancel
	defer cancel()

	startTime := time.Now()
	bq.mu.Lock()
	job.StartedAt = &startTime
	job.Status = BuildStatusRunning
	bq.active[job.ID] = job
	bq.mu.Unlock()

	bq.emitBuildStartedEvent(jobCtx, job, workerID)

	err := bq.executeBuild(jobCtx, job)

	duration := bq.markJobCompleted(job, err)
	bq.emitCompletionEvents(ctx, job, err, duration)
}

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
		slog.Warn("Failed to emit BuildStarted event", "job_id", job.ID, "err", err)
	}
}

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

	return duration
}

func (bq *BuildQueue) emitCompletionEvents(ctx context.Context, job *BuildJob, err error, duration time.Duration) {
	if bq.eventEmitter == nil {
		return
	}

	report := bq.extractBuildReport(job)
	bq.emitBuildReportEvent(ctx, job, report)

	if err != nil {
		bq.emitBuildFailedEvent(ctx, job, err)
		return
	}
	bq.emitBuildCompletedEvent(ctx, job, duration, report)
}

func (bq *BuildQueue) extractBuildReport(job *BuildJob) *models.BuildReport {
	if job.TypedMeta != nil && job.TypedMeta.BuildReport != nil {
		return job.TypedMeta.BuildReport
	}
	return nil
}

func (bq *BuildQueue) emitBuildReportEvent(ctx context.Context, job *BuildJob, report *models.BuildReport) {
	if report == nil {
		return
	}
	if err := bq.eventEmitter.EmitBuildReport(ctx, job.ID, report); err != nil {
		slog.Warn("Failed to emit BuildReport event", "job_id", job.ID, "err", err)
	}
}

func (bq *BuildQueue) emitBuildFailedEvent(ctx context.Context, job *BuildJob, err error) {
	if emitErr := bq.eventEmitter.EmitBuildFailed(ctx, job.ID, "build", err.Error()); emitErr != nil {
		slog.Warn("Failed to emit BuildFailed event", "job_id", job.ID, "err", emitErr)
	}
}

func (bq *BuildQueue) emitBuildCompletedEvent(ctx context.Context, job *BuildJob, duration time.Duration, report *models.BuildReport) {
	artifacts := make(map[string]string)
	if report != nil {
		artifacts["files"] = strconv.Itoa(report.Files)
		artifacts["repositories"] = strconv.Itoa(report.Repositories)
	}
	if err := bq.eventEmitter.EmitBuildCompleted(ctx, job.ID, duration, artifacts); err != nil {
		slog.Warn("Failed to emit BuildCompleted event", "job_id", job.ID, "err", err)
	}
}

func (bq *BuildQueue) addToHistory(job *BuildJob) {
	bq.history = append(bq.history, job)
	if len(bq.history) > bq.historySize {
		copy(bq.history, bq.history[len(bq.history)-bq.historySize:])
		bq.history = bq.history[:bq.historySize]
	}
}

func (bq *BuildQueue) executeBuild(ctx context.Context, job *BuildJob) error {
	policy := bq.retryPolicy
	if policy.Initial <= 0 {
		policy = retry.DefaultPolicy()
	}

	attempts := 0
	totalRetries := 0

	for {
		attempts++
		report, err := bq.builder.Build(ctx, job)
		if report != nil {
			meta := EnsureTypedMeta(job)
			meta.BuildReport = report
		}
		if err == nil {
			if report != nil && totalRetries > 0 {
				report.Retries = totalRetries
			}
			return nil
		}

		transient, transientStage := findTransientError(report)
		if shouldStopRetrying(transient, totalRetries, policy.MaxRetries) {
			handleRetriesExhausted(report, transient, totalRetries, transientStage, bq.recorder)
			return err
		}

		totalRetries++
		if transientStage != "" {
			bq.recorder.IncBuildRetry(transientStage)
		}
		delay := policy.Delay(totalRetries)
		slog.Warn("Transient build error, retrying",
			"job_id", job.ID,
			"attempt", attempts,
			"retry", totalRetries,
			"max_retries", policy.MaxRetries,
			"stage", transientStage,
			"delay", delay,
			"err", err,
		)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func shouldStopRetrying(transient bool, totalRetries, maxRetries int) bool {
	return !transient || totalRetries >= maxRetries
}

func handleRetriesExhausted(report *models.BuildReport, transient bool, totalRetries int, transientStage string, recorder metrics.Recorder) {
	if !transient || totalRetries < 1 {
		return
	}

	if report != nil {
		report.Retries = totalRetries
		report.RetriesExhausted = true
	}
	if transientStage != "" {
		recorder.IncBuildRetryExhausted(transientStage)
	}
}

func findTransientError(report *models.BuildReport) (bool, string) {
	if report == nil || len(report.Errors) == 0 {
		return false, ""
	}

	for _, e := range report.Errors {
		var se *models.StageError
		if stdErrors.As(e, &se) && se.Transient() {
			return true, string(se.Stage)
		}
	}
	return false, ""
}
