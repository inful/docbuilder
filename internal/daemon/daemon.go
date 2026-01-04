package daemon

import (
	"context"
	// #nosec G501 -- MD5 used for content change detection, not cryptographic security
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ggit "github.com/go-git/go-git/v5"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/state"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// Status represents the current state of the daemon.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusError    Status = "error"
)

// Daemon represents the main daemon service.
type Daemon struct {
	config         *config.Config
	configFilePath string
	status         atomic.Value // DaemonStatus
	startTime      time.Time
	stopChan       chan struct{}
	mu             sync.RWMutex

	// Core components
	forgeManager *forge.Manager
	discovery    *forge.DiscoveryService
	metrics      *MetricsCollector
	httpServer   *HTTPServer
	scheduler    *Scheduler
	buildQueue   *BuildQueue
	stateManager state.DaemonStateManager
	liveReload   *LiveReloadHub

	// Event sourcing components (Phase B)
	eventStore      eventstore.Store
	buildProjection *eventstore.BuildHistoryProjection
	eventEmitter    *EventEmitter

	// Runtime state
	activeJobs  int32
	queueLength int32
	lastBuild   *time.Time

	// Discovery cache for fast status queries
	discoveryCache *DiscoveryCache

	// Discovery runner for forge discovery operations
	discoveryRunner *DiscoveryRunner

	// Link verification service
	linkVerifier *linkverify.VerificationService

	// Build status tracker for preview mode (optional, used by local preview)
	buildStatus interface{ getStatus() (bool, error, bool) }
}

// NewDaemon creates a new daemon instance
// NewDaemon creates a new daemon instance.
func NewDaemon(cfg *config.Config) (*Daemon, error) {
	return NewDaemonWithConfigFile(cfg, "")
}

// NewDaemonWithConfigFile creates a new daemon instance with config file watching.
func NewDaemonWithConfigFile(cfg *config.Config, configFilePath string) (*Daemon, error) {
	if cfg == nil {
		return nil, errors.New("configuration is required")
	}

	if cfg.Daemon == nil {
		return nil, errors.New("daemon configuration is required")
	}

	daemon := &Daemon{
		config:         cfg,
		configFilePath: configFilePath,
		stopChan:       make(chan struct{}),
		metrics:        NewMetricsCollector(),
		discoveryCache: NewDiscoveryCache(),
	}

	daemon.status.Store(StatusStopped)

	// Initialize forge manager
	forgeManager := forge.NewForgeManager()
	for _, forgeConfig := range cfg.Forges {
		client, err := forge.NewForgeClient(forgeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create forge client %s: %w", forgeConfig.Name, err)
		}
		forgeManager.AddForge(forgeConfig, client)
	}
	daemon.forgeManager = forgeManager

	// Initialize discovery service
	daemon.discovery = forge.NewDiscoveryService(forgeManager, cfg.Filtering)

	// Initialize HTTP server
	daemon.httpServer = NewHTTPServer(cfg, daemon)

	// Create canonical BuildService (Phase D - Single Execution Pipeline)
	buildService := build.NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			// Use persistent workspace for incremental builds (repo_cache_dir/working)
			return workspace.NewPersistentManager(cfg.Daemon.Storage.RepoCacheDir, "working")
		}).
		WithHugoGeneratorFactory(func(cfg any, outputDir string) build.HugoGenerator {
			// Type assert cfg to *config.Config
			configTyped, ok := cfg.(*config.Config)
			if !ok {
				slog.Error("Invalid config type passed to Hugo generator factory")
				return nil
			}
			return hugo.NewGenerator(configTyped, outputDir)
		}).
		WithSkipEvaluatorFactory(func(outputDir string) build.SkipEvaluator {
			// Create skip evaluator with state manager access
			// Will be populated after state manager is initialized
			if daemon.stateManager == nil {
				slog.Warn("Skip evaluator factory called before state manager initialized - skipping evaluation")
				return nil
			}
			gen := hugo.NewGenerator(daemon.config, outputDir)
			inner := NewSkipEvaluator(outputDir, daemon.stateManager, gen)

			// Wrap in adapter to match build.SkipEvaluator interface
			return &skipEvaluatorAdapter{inner: inner}
		})
	buildAdapter := NewBuildServiceAdapter(buildService)

	// Initialize build queue with the canonical builder
	daemon.buildQueue = NewBuildQueue(cfg.Daemon.Sync.QueueSize, cfg.Daemon.Sync.ConcurrentBuilds, buildAdapter)
	// Configure retry policy from build config (recorder injection handled elsewhere if added later)
	daemon.buildQueue.ConfigureRetry(cfg.Build)

	// Initialize scheduler (after build queue)
	scheduler, err := NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}
	daemon.scheduler = scheduler
	// Provide back-reference so scheduler can inject metadata (live reload hub, config, state)
	daemon.scheduler.SetDaemon(daemon)

	// Initialize state manager using the typed state.Service wrapped in ServiceAdapter.
	// This bridges the new typed state system with the daemon's interface requirements.
	stateDir := cfg.Daemon.Storage.RepoCacheDir
	if stateDir == "" {
		stateDir = "./daemon-data" // Default data directory
	}
	stateServiceResult := state.NewService(stateDir)
	if stateServiceResult.IsErr() {
		return nil, fmt.Errorf("failed to create state service: %w", stateServiceResult.UnwrapErr())
	}
	daemon.stateManager = state.NewServiceAdapter(stateServiceResult.Unwrap())

	// Initialize event store and build history projection (Phase B - Event Sourcing)
	eventStorePath := filepath.Join(stateDir, "events.db")
	eventStore, err := eventstore.NewSQLiteStore(eventStorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}
	daemon.eventStore = eventStore
	daemon.buildProjection = eventstore.NewBuildHistoryProjection(eventStore, 100)
	daemon.eventEmitter = NewEventEmitter(eventStore, daemon.buildProjection)
	daemon.eventEmitter.daemon = daemon // Wire back reference for hooks

	// Rebuild projection from existing events
	if err := daemon.buildProjection.Rebuild(context.Background()); err != nil {
		slog.Warn("Failed to rebuild build history projection", logfields.Error(err))
		// Non-fatal: projection will start empty
	}

	// Initialize livereload hub (opt-in)
	if cfg.Build.LiveReload {
		daemon.liveReload = NewLiveReloadHub(daemon.metrics)
		slog.Info("LiveReload hub initialized")
	}

	// Initialize link verification service if enabled
	if cfg.Daemon.LinkVerification != nil && cfg.Daemon.LinkVerification.Enabled {
		linkVerifier, err := linkverify.NewVerificationService(cfg.Daemon.LinkVerification)
		if err != nil {
			slog.Warn("Failed to initialize link verification service",
				logfields.Error(err),
				"enabled", false)
		} else {
			daemon.linkVerifier = linkVerifier
			slog.Info("Link verification service initialized",
				"nats_url", cfg.Daemon.LinkVerification.NATSURL,
				"kv_bucket", cfg.Daemon.LinkVerification.KVBucket)
		}
	}

	// Wire up event emitter for build queue (Phase B)
	daemon.buildQueue.SetEventEmitter(daemon.eventEmitter)

	// Initialize discovery runner (Phase H - extracted component)
	daemon.discoveryRunner = NewDiscoveryRunner(DiscoveryRunnerConfig{
		Discovery:      daemon.discovery,
		ForgeManager:   daemon.forgeManager,
		DiscoveryCache: daemon.discoveryCache,
		Metrics:        daemon.metrics,
		StateManager:   daemon.stateManager,
		BuildQueue:     daemon.buildQueue,
		LiveReload:     daemon.liveReload,
		Config:         cfg,
	})

	return daemon, nil
}

// defaultDaemonInstance is used by optional Prometheus integration to pull metrics
// into the Prometheus registry when the build tag is enabled.
var defaultDaemonInstance *Daemon

// Start starts the daemon and all its components.
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.GetStatus() != StatusStopped {
		d.mu.Unlock()
		return fmt.Errorf("daemon is not in stopped state: %s", d.GetStatus())
	}

	d.status.Store(StatusStarting)
	d.startTime = time.Now()

	// Initialize metrics
	d.metrics.IncrementCounter("daemon_starts")
	d.metrics.SetGauge("daemon_status", int64(1)) // 1 = starting

	// Set global reference for metrics bridge (prometheus build only uses it).
	defaultDaemonInstance = d
	slog.Info("Starting DocBuilder daemon", slog.String("version", "2.0"))

	// Load persistent state
	if err := d.stateManager.Load(); err != nil {
		slog.Warn("Failed to load state", "error", err)
	}

	// Start HTTP servers
	if err := d.httpServer.Start(ctx); err != nil {
		d.status.Store(StatusError)
		d.mu.Unlock()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start build queue processing
	d.buildQueue.Start(ctx)

	// Start scheduler
	d.scheduler.Start(ctx)

	d.status.Store(StatusRunning)
	d.metrics.SetGauge("daemon_status", int64(2)) // 2 = running
	d.metrics.IncrementCounter("daemon_successful_starts")

	slog.Info("DocBuilder daemon started successfully",
		slog.Int("forges", len(d.config.Forges)),
		slog.Int("docs_port", d.config.Daemon.HTTP.DocsPort),
		slog.Int("admin_port", d.config.Daemon.HTTP.AdminPort),
		slog.Int("webhook_port", d.config.Daemon.HTTP.WebhookPort))

	// Emit a storage/workspace summary so operators understand path roles.
	var (
		repoCache = ""
		outDir    = d.config.Output.Directory
		wsPredict string
	)
	if d.config.Daemon != nil {
		repoCache = d.config.Daemon.Storage.RepoCacheDir
	}
	if outDir == "" {
		outDir = "./site"
	}
	strategy := d.config.Build.CloneStrategy
	if strategy == "" {
		strategy = config.CloneStrategyFresh
	}
	// Predict default workspace resolution (may differ per build if user overrides build.workspace_dir).
	switch {
	case d.config.Build.WorkspaceDir != "":
		wsPredict = d.config.Build.WorkspaceDir + " (configured)"
	case strategy == config.CloneStrategyFresh:
		wsPredict = filepath.Join(outDir, "_workspace") + " (ephemeral)"
	case repoCache != "":
		wsPredict = filepath.Join(repoCache, "working") + " (persistent via repo_cache_dir)"
	default:
		wsPredict = filepath.Clean(outDir+"-workspace") + " (persistent sibling)"
	}
	slog.Info("Storage paths summary",
		slog.String("output_dir", outDir),
		slog.String("repo_cache_dir", repoCache),
		slog.String("workspace_resolved", wsPredict),
		slog.String("clone_strategy", string(strategy)))

	// Release lock before entering long-running loop to avoid blocking read operations (e.g., /status)
	d.mu.Unlock()

	// Run main daemon loop (blocks until stopped)
	d.mainLoop(ctx)

	// When mainLoop exits, we're stopping
	d.status.Store(StatusStopping)
	slog.Info("Main loop exited, daemon stopping")

	return nil
}

// Stop gracefully shuts down the daemon.
func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	currentStatus := d.GetStatus()
	if currentStatus == StatusStopped || currentStatus == StatusStopping {
		return nil
	}

	d.status.Store(StatusStopping)
	slog.Info("Stopping DocBuilder daemon")

	// Signal stop to all components (only if not already closed)
	select {
	case <-d.stopChan:
		// Channel already closed
	default:
		close(d.stopChan)
	}

	// Stop components in reverse order
	if d.scheduler != nil {
		if err := d.scheduler.Stop(ctx); err != nil {
			slog.Error("Failed to stop scheduler", logfields.Error(err))
		}
	}

	if d.buildQueue != nil {
		d.buildQueue.Stop(ctx)
	}

	if d.httpServer != nil {
		if err := d.httpServer.Stop(ctx); err != nil {
			slog.Error("Failed to stop HTTP server", "error", err)
		}
	}

	if d.liveReload != nil {
		d.liveReload.Shutdown()
	}

	// Close link verification service
	if d.linkVerifier != nil {
		if err := d.linkVerifier.Close(); err != nil {
			slog.Error("Failed to close link verifier", logfields.Error(err))
		}
	}

	// Save state
	if d.stateManager != nil {
		if err := d.stateManager.Save(); err != nil {
			slog.Error("Failed to save state", "error", err)
		}
	}

	// Close event store (Phase B)
	if d.eventStore != nil {
		if err := d.eventStore.Close(); err != nil {
			slog.Error("Failed to close event store", logfields.Error(err))
		}
	}

	d.status.Store(StatusStopped)

	uptime := time.Since(d.startTime)
	slog.Info("DocBuilder daemon stopped", slog.Duration("uptime", uptime))

	return nil
}

// GetStatus returns the current daemon status.
func (d *Daemon) GetStatus() Status {
	status, ok := d.status.Load().(Status)
	if !ok {
		return StatusError
	}
	return status
}

// GetActiveJobs returns the number of active build jobs.
func (d *Daemon) GetActiveJobs() int {
	return int(atomic.LoadInt32(&d.activeJobs))
}

// GetQueueLength returns the current build queue length.
func (d *Daemon) GetQueueLength() int {
	return int(atomic.LoadInt32(&d.queueLength))
}

// GetStartTime returns the daemon start time.
func (d *Daemon) GetStartTime() time.Time {
	return d.startTime
}

// GetBuildProjection returns the build history projection for querying build history.
// Returns nil if event sourcing is not initialized.
func (d *Daemon) GetBuildProjection() *eventstore.BuildHistoryProjection {
	return d.buildProjection
}

// EmitBuildEvent persists an event to the event store and updates the projection.
// This delegates to the eventEmitter component.
func (d *Daemon) EmitBuildEvent(ctx context.Context, event eventstore.Event) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitEvent(ctx, event)
}

// EmitBuildStarted implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildStarted(ctx context.Context, buildID string, meta eventstore.BuildStartedMeta) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildStarted(ctx, buildID, meta)
}

// EmitBuildCompleted implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildCompleted(ctx context.Context, buildID string, duration time.Duration, artifacts map[string]string) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildCompleted(ctx, buildID, duration, artifacts)
}

// EmitBuildFailed implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildFailed(ctx context.Context, buildID, stage, errorMsg string) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildFailed(ctx, buildID, stage, errorMsg)
}

// onBuildReportEmitted is called after a build report is emitted to the event store.
// This is where we trigger post-build hooks like link verification and state updates.
func (d *Daemon) onBuildReportEmitted(ctx context.Context, buildID string, report *hugo.BuildReport) error {
	// Update state manager after successful builds
	// This is critical for skip evaluation to work correctly on subsequent builds
	if report != nil && report.Outcome == hugo.OutcomeSuccess && d.stateManager != nil && d.config != nil {
		d.updateStateAfterBuild(report)
	}

	// Trigger link verification after successful builds (low priority background task)
	slog.Debug("onBuildReportEmitted called",
		"build_id", buildID,
		"report_nil", report == nil,
		"outcome", func() string {
			if report != nil {
				return string(report.Outcome)
			}
			return "N/A"
		}(),
		"verifier_nil", d.linkVerifier == nil)
	if report != nil && report.Outcome == hugo.OutcomeSuccess && d.linkVerifier != nil {
		go d.verifyLinksAfterBuild(ctx, buildID)
	}

	return nil
}

// EmitBuildReport implements BuildEventEmitter for the daemon (legacy/compatibility).
// This is now handled by EventEmitter calling onBuildReportEmitted.
func (d *Daemon) EmitBuildReport(ctx context.Context, buildID string, report *hugo.BuildReport) error {
	// Delegate to event emitter which will call back to onBuildReportEmitted
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildReport(ctx, buildID, report)
}

// updateStateAfterBuild updates the state manager with build metadata for skip evaluation.
// This ensures subsequent builds can correctly detect when nothing has changed.
func (d *Daemon) updateStateAfterBuild(report *hugo.BuildReport) {
	// Update config hash
	if report.ConfigHash != "" {
		d.stateManager.SetLastConfigHash(report.ConfigHash)
		slog.Debug("Updated config hash in state", "hash", report.ConfigHash)
	}

	// Update global doc files hash
	if report.DocFilesHash != "" {
		d.stateManager.SetLastGlobalDocFilesHash(report.DocFilesHash)
		slog.Debug("Updated global doc files hash in state", "hash", report.DocFilesHash)
	}

	// Update repository commits and hashes
	// Read from persistent workspace (repo_cache_dir/working) to get current commit SHAs
	workspacePath := filepath.Join(d.config.Daemon.Storage.RepoCacheDir, "working")
	for i := range d.config.Repositories {
		repo := &d.config.Repositories[i]
		repoPath := filepath.Join(workspacePath, repo.Name)

		// Check if repository exists
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
			continue // Skip if not a git repository
		}

		// Open git repository to get current commit
		gitRepo, err := ggit.PlainOpen(repoPath)
		if err != nil {
			slog.Warn("Failed to open git repository for state update",
				"repository", repo.Name,
				"path", repoPath,
				"error", err)
			continue
		}

		// Get HEAD reference
		ref, err := gitRepo.Head()
		if err != nil {
			slog.Warn("Failed to get HEAD for state update",
				"repository", repo.Name,
				"error", err)
			continue
		}

		commit := ref.Hash().String()

		// Initialize repository state if needed
		d.stateManager.EnsureRepositoryState(repo.URL, repo.Name, repo.Branch)

		// Update commit in state
		d.stateManager.SetRepoLastCommit(repo.URL, repo.Name, repo.Branch, commit)
		slog.Debug("Updated repository commit in state",
			"repository", repo.Name,
			"commit", commit[:8])
	}

	// Save state to disk
	if err := d.stateManager.Save(); err != nil {
		slog.Warn("Failed to save state after build", "error", err)
	}
}

// verifyLinksAfterBuild runs link verification in the background after a successful build.
// This is a low-priority task that doesn't block the build pipeline.
func (d *Daemon) verifyLinksAfterBuild(ctx context.Context, buildID string) {
	// Create background context with timeout (derived from parent ctx)
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	slog.Info("Starting link verification for build", "build_id", buildID)

	// Collect page metadata from build report
	pages, err := d.collectPageMetadata(buildID)
	if err != nil {
		slog.Error("Failed to collect page metadata for link verification",
			"build_id", buildID,
			"error", err)
		return
	}

	// Verify links
	if err := d.linkVerifier.VerifyPages(verifyCtx, pages); err != nil {
		slog.Warn("Link verification encountered errors",
			"build_id", buildID,
			"error", err)
		return
	}

	slog.Info("Link verification completed successfully", "build_id", buildID)
}

// collectPageMetadata collects metadata for all pages in the build.
func (d *Daemon) collectPageMetadata(buildID string) ([]*linkverify.PageMetadata, error) {
	outputDir := d.config.Daemon.Storage.OutputDir
	publicDir := filepath.Join(outputDir, "public")

	var pages []*linkverify.PageMetadata

	// Walk the public directory to find all HTML files
	err := filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process HTML files
		if info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Get relative path from public directory
		relPath, err := filepath.Rel(publicDir, path)
		if err != nil {
			return err
		}

		// Create basic DocFile structure (we don't have the original here)
		// The link verifier mostly needs the path information
		docFile := &docs.DocFile{
			Path:         path,
			RelativePath: relPath,
			Repository:   extractRepoFromPath(relPath),
			Name:         strings.TrimSuffix(filepath.Base(path), ".html"),
		}

		// Try to find corresponding content file to extract front matter
		var frontMatter map[string]any
		contentPath := filepath.Join(outputDir, "content", strings.TrimSuffix(relPath, ".html")+".md")
		if contentBytes, err := os.ReadFile(filepath.Clean(contentPath)); err == nil {
			if fm, err := linkverify.ParseFrontMatter(contentBytes); err == nil {
				frontMatter = fm
			}
		}

		// Build rendered URL
		renderedURL := d.config.Hugo.BaseURL
		if !strings.HasSuffix(renderedURL, "/") {
			renderedURL += "/"
		}
		renderedURL += strings.TrimPrefix(relPath, "/")

		// Compute MD5 hash of HTML content for change detection
		var contentHash string
		if htmlBytes, err := os.ReadFile(filepath.Clean(path)); err == nil {
			// #nosec G401 -- MD5 is used for content hashing, not cryptographic security
			hash := md5.New()
			hash.Write(htmlBytes)
			contentHash = hex.EncodeToString(hash.Sum(nil))
		}

		page := &linkverify.PageMetadata{
			DocFile:      docFile,
			HTMLPath:     path,
			HugoPath:     contentPath,
			RenderedPath: relPath,
			RenderedURL:  renderedURL,
			FrontMatter:  frontMatter,
			BaseURL:      d.config.Hugo.BaseURL,
			BuildID:      buildID,
			BuildTime:    time.Now(),
			ContentHash:  contentHash,
		}

		pages = append(pages, page)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk public directory: %w", err)
	}

	slog.Debug("Collected page metadata for link verification",
		"build_id", buildID,
		"page_count", len(pages))

	return pages, nil
}

// extractRepoFromPath attempts to extract repository name from rendered path.
// Rendered paths typically follow pattern: repo-name/section/file.html
// Hugo-generated pages (categories, tags, etc.) are marked with "_hugo" prefix.
func extractRepoFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) == 0 {
		return "unknown"
	}

	firstSegment := parts[0]

	// Recognize Hugo-generated taxonomy and special pages
	if isHugoGeneratedPath(firstSegment) {
		return "_hugo_" + firstSegment
	}

	// For root-level files (index.html, 404.html, sitemap.xml, etc.)
	if len(parts) == 1 {
		return "_hugo_root"
	}

	return firstSegment
}

// isHugoGeneratedPath checks if a path segment is a Hugo-generated taxonomy or special page.
func isHugoGeneratedPath(segment string) bool {
	HugoGeneratedPaths := map[string]bool{
		"categories":  true,
		"tags":        true,
		"authors":     true,
		"series":      true,
		"search":      true,
		"sitemap.xml": true,
	}
	return HugoGeneratedPaths[segment]
}

// Compile-time check that Daemon implements BuildEventEmitter.
var _ BuildEventEmitter = (*Daemon)(nil)

// TriggerDiscovery manually triggers repository discovery.
func (d *Daemon) TriggerDiscovery() string {
	return d.discoveryRunner.TriggerManual(d.GetStatus, &d.activeJobs)
}

// TriggerBuild manually triggers a site build.
func (d *Daemon) TriggerBuild() string {
	if d.GetStatus() != StatusRunning {
		return ""
	}

	jobID := fmt.Sprintf("build-%d", time.Now().Unix())

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeManual,
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
		},
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue build job", logfields.JobID(jobID), logfields.Error(err))
		return ""
	}

	slog.Info("Manual build triggered", logfields.JobID(jobID))
	return jobID
}

// TriggerWebhookBuild triggers a build for specific repositories from a webhook event.
// This allows targeted rebuilds without refetching all repositories.
func (d *Daemon) TriggerWebhookBuild(repoFullName, branch string) string {
	if d.GetStatus() != StatusRunning {
		return ""
	}

	// Find matching repository in config
	var targetRepos []config.Repository
	for i := range d.config.Repositories {
		repo := &d.config.Repositories[i]
		// Match by name or full name extracted from URL
		// GitHub URL format: https://github.com/owner/repo.git or git@github.com:owner/repo.git
		// GitLab URL format: https://gitlab.com/owner/repo.git or git@gitlab.com:owner/repo.git
		// Forgejo URL format: https://git.home.luguber.info/owner/repo.git or git@git.home.luguber.info:owner/repo.git
		if repo.Name == repoFullName || matchesRepoURL(repo.URL, repoFullName) {
			// If branch is specified, only rebuild if it matches the configured branch
			if branch == "" || repo.Branch == branch {
				targetRepos = append(targetRepos, *repo)
				slog.Info("Webhook matched repository",
					"repo", repo.Name,
					"full_name", repoFullName,
					"branch", branch)
			}
		}
	}

	if len(targetRepos) == 0 {
		slog.Warn("No matching repositories found for webhook",
			"repo_full_name", repoFullName,
			"branch", branch)
		return ""
	}

	jobID := fmt.Sprintf("webhook-%d", time.Now().Unix())

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeWebhook,
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			Repositories:  targetRepos,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
			DeltaRepoReasons: map[string]string{
				repoFullName: fmt.Sprintf("webhook push to %s", branch),
			},
		},
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue webhook build job", logfields.JobID(jobID), logfields.Error(err))
		return ""
	}

	slog.Info("Webhook build triggered",
		logfields.JobID(jobID),
		"repo", repoFullName,
		"branch", branch,
		"target_count", len(targetRepos))

	atomic.AddInt32(&d.queueLength, 1)
	return jobID
}

// matchesRepoURL checks if a repository URL matches the given full name (owner/repo).
func matchesRepoURL(repoURL, fullName string) bool {
	// Extract owner/repo from various URL formats:
	// - https://github.com/owner/repo.git
	// - git@github.com:owner/repo.git
	// - https://github.com/owner/repo
	// - git@github.com:owner/repo

	// Remove trailing .git if present
	url := repoURL
	if len(url) > 4 && url[len(url)-4:] == ".git" {
		url = url[:len(url)-4]
	}

	// Check if URL ends with the full name
	if len(url) > len(fullName) {
		// Check for /owner/repo or :owner/repo
		if url[len(url)-len(fullName)-1] == '/' || url[len(url)-len(fullName)-1] == ':' {
			if url[len(url)-len(fullName):] == fullName {
				return true
			}
		}
	}

	return false
}

// triggerScheduledBuildForExplicitRepos triggers a scheduled build for explicitly configured repositories.
func (d *Daemon) triggerScheduledBuildForExplicitRepos() {
	if d.GetStatus() != StatusRunning {
		return
	}

	jobID := fmt.Sprintf("scheduled-build-%d", time.Now().Unix())

	slog.Info("Triggering scheduled build for explicit repositories",
		logfields.JobID(jobID),
		slog.Int("repositories", len(d.config.Repositories)))

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeScheduled,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			Repositories:  d.config.Repositories,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
		},
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue scheduled build", logfields.JobID(jobID), logfields.Error(err))
		return
	}

	atomic.AddInt32(&d.queueLength, 1)
}

// mainLoop runs the main daemon processing loop.
func (d *Daemon) mainLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Status update interval
	defer ticker.Stop()

	// Discovery schedule: run initial after short delay, then every configured interval (default 10m).
	discoveryInterval := 10 * time.Minute
	if d.config != nil && d.config.Daemon != nil {
		if expr := strings.TrimSpace(d.config.Daemon.Sync.Schedule); expr != "" {
			if parsed, ok := parseDiscoverySchedule(expr); ok {
				discoveryInterval = parsed
				slog.Info("Configured discovery schedule", slog.String("expression", expr), slog.Duration("interval", discoveryInterval))
			} else {
				slog.Warn("Unrecognized discovery schedule expression; falling back to default", slog.String("expression", expr), slog.Duration("fallback_interval", discoveryInterval))
			}
		}
	}
	discoveryTicker := time.NewTicker(discoveryInterval)
	defer discoveryTicker.Stop()

	initialDiscoveryTimer := time.NewTimer(3 * time.Second)
	defer initialDiscoveryTimer.Stop()

	// If explicit repositories are configured (no forges), trigger an immediate build
	if len(d.config.Repositories) > 0 && len(d.config.Forges) == 0 {
		slog.Info("Explicit repositories configured, triggering initial build", slog.Int("repositories", len(d.config.Repositories)))
		go func() {
			// Trigger build with explicit repositories
			job := &BuildJob{
				ID:        fmt.Sprintf("initial-build-%d", time.Now().Unix()),
				Type:      BuildTypeManual,
				Priority:  PriorityNormal,
				CreatedAt: time.Now(),
				TypedMeta: &BuildJobMetadata{
					V2Config:      d.config,
					Repositories:  d.config.Repositories,
					StateManager:  d.stateManager,
					LiveReloadHub: d.liveReload,
				},
			}
			if err := d.buildQueue.Enqueue(job); err != nil {
				slog.Error("Failed to enqueue initial build", logfields.Error(err))
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Main loop stopped by context cancellation")
			return
		case <-d.stopChan:
			slog.Info("Main loop stopped by stop signal")
			return
		case <-ticker.C:
			d.updateStatus()
		case <-initialDiscoveryTimer.C:
			go d.discoveryRunner.SafeRun(ctx, d.GetStatus)
		case <-discoveryTicker.C:
			slog.Info("Scheduled tick", slog.Duration("interval", discoveryInterval))
			// For forge-based discovery, run discovery
			if len(d.config.Forges) > 0 {
				go d.discoveryRunner.SafeRun(ctx, d.GetStatus)
			}
			// For explicit repositories, trigger a build to check for updates
			if len(d.config.Repositories) > 0 {
				go d.triggerScheduledBuildForExplicitRepos()
			}
		}
	}
}

// parseDiscoverySchedule parses a schedule expression into an approximate interval.
// Supported forms:
//
//	@every <duration>   (same semantics as Go duration parsing, e.g. @every 5m, @every 1h30m)
//	Standard 5-field cron patterns (minute hour day month weekday) for a few common forms:
//	  */5 * * * *   -> 5m
//	  */15 * * * *  -> 15m
//	  0 * * * *     -> 1h (top of every hour)
//	  0 0 * * *     -> 24h (midnight daily)
//	  */30 * * * *  -> 30m
//	If expression not recognized returns (0,false).
func parseDiscoverySchedule(expr string) (time.Duration, bool) {
	// @every form
	if after, ok := strings.CutPrefix(expr, "@every "); ok {
		rem := strings.TrimSpace(after)
		if d, err := time.ParseDuration(rem); err == nil && d > 0 {
			return d, true
		}
		return 0, false
	}
	parts := strings.Fields(expr)
	if len(parts) != 5 { // not a simplified cron pattern we support
		return 0, false
	}
	switch expr {
	case "*/5 * * * *":
		return 5 * time.Minute, true
	case "*/15 * * * *":
		return 15 * time.Minute, true
	case "*/30 * * * *":
		return 30 * time.Minute, true
	case "0 * * * *":
		return time.Hour, true
	case "0 0 * * *":
		return 24 * time.Hour, true
	default:
		// Attempt to parse expressions like "*/10 * * * *"
		if after, ok := strings.CutPrefix(parts[0], "*/"); ok {
			val := after
			if n, err := strconv.Atoi(val); err == nil && n > 0 && n < 60 {
				return time.Duration(n) * time.Minute, true
			}
		}
	}
	return 0, false
}

// updateStatus updates runtime status and metrics.
func (d *Daemon) updateStatus() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update queue length from build queue
	if d.buildQueue != nil {
		// Clamp to int32 range to avoid overflow warnings from linters and ensure atomic store safety
		n := d.buildQueue.Length()
		if n > math.MaxInt32 {
			n = math.MaxInt32
		} else if n < math.MinInt32 {
			n = math.MinInt32
		}
		// #nosec G115 -- value is clamped to int32 range above
		atomic.StoreInt32(&d.queueLength, int32(n))
	} // Periodic state save
	if d.stateManager != nil {
		if err := d.stateManager.Save(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}
	}
}

// GetConfig returns the current daemon configuration.
func (d *Daemon) GetConfig() *config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}
