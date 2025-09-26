package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
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
	return bq.performSiteBuild(ctx, job)
}

// executeScheduledBuild handles cron-triggered builds
func (bq *BuildQueue) executeScheduledBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing scheduled build", "job_id", job.ID)
	return bq.performSiteBuild(ctx, job)
}

// executeWebhookBuild handles webhook-triggered builds
func (bq *BuildQueue) executeWebhookBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing webhook build", "job_id", job.ID)
	return bq.performSiteBuild(ctx, job)
}

// executeDiscoveryBuild handles auto-builds after discovery
func (bq *BuildQueue) executeDiscoveryBuild(ctx context.Context, job *BuildJob) error {
	slog.Info("Executing discovery build", "job_id", job.ID)
	return bq.performSiteBuild(ctx, job)
}

// performSiteBuild encapsulates the actual build process: cloning repositories, discovering docs,
// generating the Hugo site, and running a static render. This is invoked for all build types.
func (bq *BuildQueue) performSiteBuild(ctx context.Context, job *BuildJob) error {
	// Access daemon via closure capturing (BuildQueue currently doesn't have direct pointer to daemon config).
	// We rely on job.Metadata carrying a *config.V2Config reference injected by the daemon when enqueuing.
	rawCfg, ok := job.Metadata["v2_config"].(*cfg.V2Config)
	if !ok || rawCfg == nil {
		return fmt.Errorf("missing v2 configuration in job metadata")
	}

	// Prepare output directory
	outDir := rawCfg.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}

	// Clean output if requested (daemon defaults enforce clean true)
	if rawCfg.Output.Clean {
		if err := os.RemoveAll(outDir); err != nil {
			slog.Warn("Failed to clean output directory", "dir", outDir, "error", err)
		}
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build a temporary workspace for cloning
	workspaceRoot := filepath.Join(outDir, "_workspace")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	gitClient := git.NewClient(workspaceRoot)
	if err := gitClient.EnsureWorkspace(); err != nil {
		return fmt.Errorf("failed to ensure workspace: %w", err)
	}

	reposAny, ok := job.Metadata["repositories"].([]cfg.Repository)
	if !ok {
		// Try slice of interface (possible encoding differences)
		if ra, ok2 := job.Metadata["repositories"].([]interface{}); ok2 {
			casted := make([]cfg.Repository, 0, len(ra))
			for _, v := range ra {
				if r, ok3 := v.(cfg.Repository); ok3 {
					casted = append(casted, r)
				}
			}
			reposAny = casted
		}
	}
	if len(reposAny) == 0 {
		slog.Warn("No repositories in job metadata; proceeding with empty set")
	}

	repoPaths := make(map[string]string, len(reposAny))
	for _, r := range reposAny {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		slog.Info("Cloning repository", "name", r.Name, "url", r.URL)
		p, err := gitClient.CloneRepository(r)
		if err != nil {
			slog.Error("Repository clone failed", "name", r.Name, "error", err)
			// track failed clone in build report if available
			if job.Metadata != nil {
				if brRaw, ok := job.Metadata["build_report"]; ok {
					if br, ok2 := brRaw.(*hugo.BuildReport); ok2 && br != nil {
						br.FailedRepositories++
					}
				}
			}
			continue
		}
		repoPaths[r.Name] = p
		if job.Metadata != nil {
			if brRaw, ok := job.Metadata["build_report"]; ok {
				if br, ok2 := brRaw.(*hugo.BuildReport); ok2 && br != nil {
					br.ClonedRepositories++
				}
			}
		}
	}

	// Pre-create a BuildReport and attach to metadata so cloning can update counts
	if job.Metadata == nil { job.Metadata = make(map[string]interface{}) }
	initialReport := hugo.NewGenerator(&cfg.Config{}, outDir) // temporary generator just to get a report? We'll instead create minimal report
	_ = initialReport // placeholder (not used) – will be replaced when actual generation occurs

	discovery := docs.NewDiscovery(reposAny)
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return fmt.Errorf("documentation discovery failed: %w", err)
	}
	slog.Info("Documentation discovery complete", "files", len(docFiles))

	// Convert v2 config to legacy config.Config expected by Hugo generator
	legacy := &cfg.Config{
		Hugo: cfg.HugoConfig{
			Theme:       rawCfg.Hugo.Theme,
			BaseURL:     rawCfg.Hugo.BaseURL,
			Title:       rawCfg.Hugo.Title,
			Description: rawCfg.Hugo.Description,
			Params:      rawCfg.Hugo.Params,
			Menu:        rawCfg.Hugo.Menu,
		},
		Output: cfg.OutputConfig{
			Directory: outDir,
			Clean:     rawCfg.Output.Clean,
		},
		Repositories: reposAny,
	}

	gen := hugo.NewGenerator(legacy, outDir)

	// Force static render in daemon builds regardless of env gating
	if err := os.Setenv("DOCBUILDER_RUN_HUGO", "1"); err != nil {
		slog.Warn("Failed to set DOCBUILDER_RUN_HUGO env", "error", err)
	}
	report, err := gen.GenerateSiteWithReportContext(ctx, docFiles)
	if err != nil {
		return fmt.Errorf("hugo generation failed: %w", err)
	}

	// Store stage timings in job metadata for status/observability.
	if job.Metadata == nil {
		job.Metadata = make(map[string]interface{})
	}
	job.Metadata["build_report"] = report
	// Emit basic metrics if daemon metrics collector attached (best-effort via metadata injection earlier)
	if mcAny, ok := job.Metadata["metrics_collector"]; ok {
		if mc, ok2 := mcAny.(interface{ IncrementCounter(string) }); ok2 {
			mc.IncrementCounter("build_completed_total")
			switch report.Outcome {
			case "failed":
				mc.IncrementCounter("build_failed_total")
			case "warning":
				mc.IncrementCounter("build_warning_total")
			case "canceled":
				mc.IncrementCounter("build_canceled_total")
			case "success":
				mc.IncrementCounter("build_success_total")
			}
		}
	}

	slog.Info("Site build completed", "output", outDir, "public_exists", dirExists(filepath.Join(outDir, "public")))
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
