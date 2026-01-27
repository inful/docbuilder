package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/server/handlers"
	"git.home.luguber.info/inful/docbuilder/internal/server/httpserver"
	"git.home.luguber.info/inful/docbuilder/internal/state"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// Status represents the current state of the daemon.
//
// Note: this is a type alias (not a distinct type) so that Daemon.GetStatus()
// satisfies interfaces that expect a plain string status.
type Status = string

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
	runCancel      context.CancelFunc
	mu             sync.RWMutex

	// Core components
	forgeManager *forge.Manager
	discovery    *forge.DiscoveryService
	metrics      *MetricsCollector
	httpServer   *httpserver.Server
	scheduler    *Scheduler
	buildQueue   *BuildQueue
	stateManager state.DaemonStateManager
	liveReload   *LiveReloadHub

	// Orchestration event bus (ADR-021; in-process control flow)
	orchestrationBus *events.Bus
	buildDebouncer   *BuildDebouncer
	repoUpdater      *RepoUpdater

	// Event sourcing components (Phase B)
	eventStore      eventstore.Store
	buildProjection *eventstore.BuildHistoryProjection
	eventEmitter    *EventEmitter

	// Runtime state
	activeJobs  int32
	queueLength int32
	lastBuild   *time.Time

	// Background worker tracking (started in Start, awaited in Stop).
	workers sync.WaitGroup

	// Scheduled job IDs (for observability and tests)
	syncJobID   string
	statusJobID string
	promJobID   string

	// Discovery cache for fast status queries
	discoveryCache *DiscoveryCache

	// Discovery runner for forge discovery operations
	discoveryRunner *DiscoveryRunner

	// Link verification service
	linkVerifier *linkverify.VerificationService
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
		config:           cfg,
		configFilePath:   configFilePath,
		stopChan:         make(chan struct{}),
		metrics:          NewMetricsCollector(),
		discoveryCache:   NewDiscoveryCache(),
		orchestrationBus: events.NewBus(),
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

	// Create canonical BuildService (Phase D - Single Execution Pipeline)
	buildService := build.NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			// Use persistent workspace for incremental builds (repo_cache_dir/working)
			return workspace.NewPersistentManager(cfg.Daemon.Storage.RepoCacheDir, "working")
		}).
		WithHugoGeneratorFactory(func(cfg *config.Config, outputDir string) build.HugoGenerator {
			return hugo.NewGenerator(cfg, outputDir)
		}).
		WithSkipEvaluatorFactory(func(outputDir string) build.SkipEvaluator {
			// Create skip evaluator with state manager access
			// Will be populated after state manager is initialized
			if daemon.stateManager == nil {
				slog.Warn("Skip evaluator factory called before state manager initialized - skipping evaluation")
				return nil
			}
			gen := hugo.NewGenerator(daemon.config, outputDir)
			return NewSkipEvaluator(outputDir, daemon.stateManager, gen)
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
	if rebuildErr := daemon.buildProjection.Rebuild(context.Background()); rebuildErr != nil {
		slog.Warn("Failed to rebuild build history projection", logfields.Error(rebuildErr))
		// Non-fatal: projection will start empty
	}

	// Initialize livereload hub (opt-in)
	if cfg.Build.LiveReload {
		daemon.liveReload = NewLiveReloadHub(daemon.metrics)
		slog.Info("LiveReload hub initialized")
	}

	// Initialize HTTP server wiring (extracted package)
	webhookConfigs := make(map[string]*config.WebhookConfig)
	for _, forgeCfg := range cfg.Forges {
		if forgeCfg == nil {
			continue
		}
		if forgeCfg.Webhook != nil {
			webhookConfigs[forgeCfg.Name] = forgeCfg.Webhook
		}
	}
	forgeClients := make(map[string]forge.Client)
	if daemon.forgeManager != nil {
		maps.Copy(forgeClients, daemon.forgeManager.GetAllForges())
	}
	var detailedMetrics http.HandlerFunc
	if daemon.metrics != nil {
		detailedMetrics = daemon.metrics.MetricsHandler
	}
	statusHandlers := handlers.NewStatusPageHandlers(daemon)
	daemon.httpServer = httpserver.New(cfg, daemon, httpserver.Options{
		ForgeClients:          forgeClients,
		WebhookConfigs:        webhookConfigs,
		LiveReloadHub:         daemon.liveReload,
		EnhancedHealthHandle:  daemon.EnhancedHealthHandler,
		DetailedMetricsHandle: detailedMetrics,
		PrometheusHandler:     prometheusOptionalHandler(),
		StatusHandle:          statusHandlers.HandleStatusPage,
	})

	// Initialize link verification service if enabled
	if cfg.Daemon.LinkVerification != nil && cfg.Daemon.LinkVerification.Enabled {
		linkVerifier, linkVerifierErr := linkverify.NewVerificationService(cfg.Daemon.LinkVerification)
		if linkVerifierErr != nil {
			slog.Warn("Failed to initialize link verification service",
				logfields.Error(linkVerifierErr),
				slog.Bool("enabled", false))
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
		BuildRequester: daemon.onDiscoveryBuildRequest,
		RepoRemoved:    daemon.onDiscoveryRepoRemoved,
		LiveReload:     daemon.liveReload,
		Config:         cfg,
	})

	// Initialize build debouncer (ADR-021 Phase 2).
	// Note: this is passive until components start publishing BuildRequested events.
	quietWindow, maxDelay, err := getBuildDebounceDurations(cfg)
	if err != nil {
		return nil, err
	}
	debouncer, err := NewBuildDebouncer(daemon.orchestrationBus, BuildDebouncerConfig{
		QuietWindow: quietWindow,
		MaxDelay:    maxDelay,
		Metrics:     daemon.metrics,
		CheckBuildRunning: func() bool {
			if daemon.buildQueue == nil {
				return false
			}
			return len(daemon.buildQueue.GetActiveJobs()) > 0
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create build debouncer: %w", err)
	}
	daemon.buildDebouncer = debouncer

	remoteCache, cacheErr := git.NewRemoteHeadCache(cfg.Daemon.Storage.RepoCacheDir)
	if cacheErr != nil {
		slog.Warn("Failed to initialize remote HEAD cache; disabling persistence", logfields.Error(cacheErr))
		remoteCache, _ = git.NewRemoteHeadCache("")
	}
	gitClient := git.NewClient(cfg.Daemon.Storage.RepoCacheDir).WithRemoteHeadCache(remoteCache)
	daemon.repoUpdater = NewRepoUpdater(daemon.orchestrationBus, gitClient, remoteCache, daemon.currentReposForOrchestratedBuild)

	return daemon, nil
}

func getBuildDebounceDurations(cfg *config.Config) (time.Duration, time.Duration, error) {
	quietWindow := 10 * time.Second
	maxDelay := 60 * time.Second
	if cfg == nil || cfg.Daemon == nil || cfg.Daemon.BuildDebounce == nil {
		return quietWindow, maxDelay, nil
	}
	if v := strings.TrimSpace(cfg.Daemon.BuildDebounce.QuietWindow); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse daemon.build_debounce.quiet_window: %w", err)
		}
		quietWindow = parsed
	}
	if v := strings.TrimSpace(cfg.Daemon.BuildDebounce.MaxDelay); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse daemon.build_debounce.max_delay: %w", err)
		}
		maxDelay = parsed
	}
	return quietWindow, maxDelay, nil
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

	// Create a derived run context that is canceled on daemon shutdown.
	runCtx, runCancel := d.workContext(ctx)
	d.runCancel = runCancel

	// Start HTTP servers
	if err := d.httpServer.Start(runCtx); err != nil {
		d.status.Store(StatusError)
		d.runCancel = nil
		runCancel()
		d.mu.Unlock()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start build queue processing
	d.buildQueue.Start(runCtx)

	d.startWorkers(runCtx)

	// Schedule periodic daemon work (cron/duration jobs) before starting the scheduler.
	if err := d.schedulePeriodicJobs(runCtx); err != nil {
		d.status.Store(StatusError)
		if d.runCancel != nil {
			d.runCancel()
			d.runCancel = nil
		}
		d.mu.Unlock()
		return fmt.Errorf("failed to schedule daemon jobs: %w", err)
	}

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
	d.mainLoop(runCtx)

	// When mainLoop exits, we're stopping
	d.status.Store(StatusStopping)
	slog.Info("Main loop exited, daemon stopping")

	return nil
}

func (d *Daemon) schedulePeriodicJobs(ctx context.Context) error {
	if d.scheduler == nil {
		return errors.New("scheduler not initialized")
	}
	if d.config == nil || d.config.Daemon == nil {
		return nil
	}

	expr := strings.TrimSpace(d.config.Daemon.Sync.Schedule)
	if expr == "" {
		// Defaults should prevent this, but keep it defensive.
		return errors.New("daemon sync schedule is empty")
	}

	syncJobID, err := d.scheduler.ScheduleCron("daemon-sync", expr, func() {
		d.runScheduledSyncTick(ctx, expr)
	})
	if err != nil {
		return err
	}
	d.syncJobID = syncJobID

	statusJobID, err := d.scheduler.ScheduleEvery("daemon-status", 30*time.Second, func() {
		if d.GetStatus() != StatusRunning {
			return
		}
		d.updateStatus()
	})
	if err != nil {
		return err
	}
	d.statusJobID = statusJobID

	// Prometheus counter bridge sync (used by /metrics handler). This replaces the
	// previous global goroutine+sleep loop so the daemon owns the periodic work.
	promJobID, err := d.scheduler.ScheduleEvery("daemon-prom-sync", 5*time.Second, func() {
		if d.GetStatus() != StatusRunning {
			return
		}
		updateDaemonPromMetrics(d)
	})
	if err != nil {
		return err
	}
	d.promJobID = promJobID

	return nil
}

func (d *Daemon) workContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	// Tie this context to daemon shutdown without storing a context on the daemon
	// itself (see linters: containedctx/contextcheck).
	go func() {
		select {
		case <-d.stopChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

func (d *Daemon) runScheduledSyncTick(ctx context.Context, expression string) {
	// Avoid running scheduled work when daemon is not running.
	if d.GetStatus() != StatusRunning {
		return
	}

	slog.Info("Scheduled sync tick", slog.String("expression", expression))

	// For forge-based discovery, run discovery.
	if len(d.config.Forges) > 0 {
		if d.discoveryRunner == nil {
			slog.Warn("Skipping scheduled discovery: discovery runner not initialized")
		} else {
			workCtx, cancel := d.workContext(ctx)
			defer cancel()
			d.discoveryRunner.SafeRun(workCtx, func() bool { return d.GetStatus() == StatusRunning })
		}
	}

	// For explicit repositories, trigger a build to check for updates.
	if len(d.config.Repositories) > 0 {
		if d.orchestrationBus == nil {
			slog.Warn("Skipping scheduled build: orchestration bus not initialized")
		} else {
			d.triggerScheduledBuildForExplicitRepos(ctx)
		}
	}
}

// Stop gracefully shuts down the daemon.
func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	currentStatus := d.GetStatus()
	if currentStatus == StatusStopped || currentStatus == StatusStopping {
		d.mu.Unlock()
		return nil
	}

	d.status.Store(StatusStopping)
	slog.Info("Stopping DocBuilder daemon")

	// Snapshot pointers so we can stop without holding the daemon mutex.
	runCancel := d.runCancel
	d.runCancel = nil
	stopChan := d.stopChan
	bus := d.orchestrationBus
	scheduler := d.scheduler
	buildQueue := d.buildQueue
	httpServer := d.httpServer
	liveReload := d.liveReload
	linkVerifier := d.linkVerifier
	stateManager := d.stateManager
	eventStore := d.eventStore
	d.mu.Unlock()

	// Cancel the run context to stop all background workers.
	if runCancel != nil {
		runCancel()
	}

	// Signal stop to all components (only if not already closed)
	if stopChan != nil {
		select {
		case <-stopChan:
			// Channel already closed
		default:
			close(stopChan)
		}
	}

	// Stop components in reverse order
	if bus != nil {
		bus.Close()
	}

	if scheduler != nil {
		if err := scheduler.Stop(ctx); err != nil {
			slog.Error("Failed to stop scheduler", logfields.Error(err))
		}
	}

	if buildQueue != nil {
		buildQueue.Stop(ctx)
	}

	if httpServer != nil {
		if err := httpServer.Stop(ctx); err != nil {
			slog.Error("Failed to stop HTTP server", "error", err)
		}
	}

	if liveReload != nil {
		liveReload.Shutdown()
	}

	// Close link verification service
	if linkVerifier != nil {
		if err := linkVerifier.Close(); err != nil {
			slog.Error("Failed to close link verifier", logfields.Error(err))
		}
	}

	// Save state
	if stateManager != nil {
		if err := stateManager.Save(); err != nil {
			slog.Error("Failed to save state", "error", err)
		}
	}

	// Close event store (Phase B)
	if eventStore != nil {
		if err := eventStore.Close(); err != nil {
			slog.Error("Failed to close event store", logfields.Error(err))
		}
	}

	// Wait for daemon-owned background workers to exit.
	done := make(chan struct{})
	go func() {
		d.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-ctx.Done():
		slog.Warn("Timed out waiting for daemon workers to stop", logfields.Error(ctx.Err()))
	}

	d.mu.Lock()
	d.status.Store(StatusStopped)
	d.mu.Unlock()

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

// Compile-time check that Daemon implements BuildEventEmitter.
var _ BuildEventEmitter = (*Daemon)(nil)
