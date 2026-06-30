package daemon

import (
	"context"
	"errors"
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
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
	queueLength atomic.Int32

	// Background worker tracking (started in Start, awaited in Stop).
	workers WorkerGroup

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
//
// This constructor wires up every subsystem in dependency order. Each
// subsystem is built by a dedicated private helper (newX) so the top-level
// function reads as a sequence of "initialize X" calls.
//
// Construction order matters: forge manager must exist before discovery
// and HTTP wiring; state service must exist before BuildService (the
// SkipEvaluatorFactory closure captures d.stateManager); BuildService
// must exist before BuildQueue (the queue wraps it); scheduler runs
// after BuildQueue so periodic jobs can target it; event store comes
// after state so the event store path joins the same state directory;
// liveReload and linkVerifier are opt-in; the HTTP server is wired
// after both; buildQueue.SetEventEmitter closes the loop between the
// queue and the emitter (must run after newEventStore); finally the
// orchestration subsystems (discovery runner, build debouncer, repo
// updater) hang off d.orchestrationBus.
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

	// Forge manager (and per-forge clients) + discovery service.
	forgeManager, err := daemon.newForgeManager()
	if err != nil {
		return nil, err
	}
	daemon.discovery = forge.NewDiscoveryService(forgeManager, cfg.Filtering)

	// State service first: SkipEvaluatorFactory closure inside
	// newBuildService captures d.stateManager and must not see nil.
	stateDir := daemon.resolveStateDir()
	if err = daemon.newStateService(stateDir); err != nil {
		return nil, err
	}

	// Build pipeline.
	buildService := daemon.newBuildService()
	daemon.newBuildQueue(buildService)

	// Scheduler runs after the build queue so periodic jobs can target it.
	daemon.scheduler, err = NewScheduler()
	if err != nil {
		return nil, derrors.WrapError(err, derrors.CategoryInternal, "failed to create scheduler").Build()
	}

	// Event sourcing.
	if err = daemon.newEventStore(stateDir); err != nil {
		return nil, err
	}

	// Opt-in subsystems.
	if cfg.Build.LiveReload {
		daemon.liveReload = NewLiveReloadHub(daemon.metrics)
		slog.Info("LiveReload hub initialized")
	}

	// HTTP server wiring.
	webhookConfigs, forgeClients := daemon.collectHTTPInputs()
	daemon.newHTTPServer(webhookConfigs, forgeClients)
	if cfg.Daemon.LinkVerification != nil && cfg.Daemon.LinkVerification.Enabled {
		daemon.initLinkVerifier()
	}

	// Close the loop between the build queue and the event emitter.
	daemon.buildQueue.SetEventEmitter(daemon.eventEmitter)

	// Orchestration: discovery runner, build debouncer, repo updater.
	daemon.newDiscoveryRunner()
	if err = daemon.newBuildDebouncer(cfg); err != nil {
		return nil, err
	}
	daemon.newRepoUpdater()

	return daemon, nil
}

// newForgeManager constructs the forge manager and per-forge clients from
// d.config.Forges. The manager is stored on d (needed by the discovery
// service, HTTP server wiring, orchestrated builds, webhook forge-name
// lookup, and health checks). Returns any error from client construction.
func (d *Daemon) newForgeManager() (*forge.Manager, error) {
	fm := forge.NewForgeManager()
	for _, fc := range d.config.Forges {
		client, err := forge.NewForgeClient(fc)
		if err != nil {
			return nil, derrors.WrapError(err, derrors.CategoryInternal, "failed to create forge client").
				WithContext("forge", fc.Name).Build()
		}
		fm.AddForge(fc, client)
	}
	d.forgeManager = fm
	return fm, nil
}

// resolveStateDir returns the directory used for daemon persistent state
// (state service + SQLite event store). Falls back to "./daemon-data" when
// the configured RepoCacheDir is empty.
func (d *Daemon) resolveStateDir() string {
	stateDir := d.config.Daemon.Storage.RepoCacheDir
	if stateDir == "" {
		return "./daemon-data"
	}
	return stateDir
}

// newStateService initializes d.stateManager using state.NewService backed
// by the given state directory. Must run before newBuildService so the
// SkipEvaluatorFactory closure captures a non-nil d.stateManager.
func (d *Daemon) newStateService(stateDir string) error {
	result := state.NewService(stateDir)
	if result.IsErr() {
		return derrors.WrapError(result.UnwrapErr(), derrors.CategoryInternal, "failed to create state service").Build()
	}
	d.stateManager = result.Unwrap()
	return nil
}

// newBuildService constructs the canonical BuildService with the workspace,
// Hugo generator, and skip evaluator factories wired to the daemon.
//
// d.stateManager must be initialized (via newStateService) before this is
// called: the skip-evaluator factory closure captures d.stateManager and
// would silently disable skip-evaluation otherwise.
func (d *Daemon) newBuildService() build.BuildService {
	return build.NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			// Use persistent workspace for incremental builds (repo_cache_dir/working).
			return workspace.NewPersistentManager(d.config.Daemon.Storage.RepoCacheDir, "working")
		}).
		WithHugoGeneratorFactory(func(cfg *config.Config, outputDir string) build.HugoGenerator {
			return hugo.NewGenerator(cfg, outputDir)
		}).
		WithSkipEvaluatorFactory(func(outputDir string) build.SkipEvaluator {
			gen := hugo.NewGenerator(d.config, outputDir)
			return NewSkipEvaluator(outputDir, d.stateManager, gen)
		})
}

// newBuildQueue initializes d.buildQueue with the adapter wrapping the
// canonical BuildService, then configures the retry policy from
// d.config.Build.
func (d *Daemon) newBuildQueue(svc build.BuildService) {
	adapter := NewBuildServiceAdapter(svc)
	d.buildQueue = NewBuildQueue(
		d.config.Daemon.Sync.QueueSize,
		d.config.Daemon.Sync.ConcurrentBuilds,
		adapter,
	)
	// Configure retry policy from build config (recorder injection handled elsewhere if added later).
	d.buildQueue.ConfigureRetry(d.config.Build)
}

// newEventStore initializes d.eventStore, d.buildProjection, and
// d.eventEmitter from a SQLite-backed store at stateDir/events.db, then
// rebuilds the build-history projection from any existing events.
//
// A failure to rebuild is logged but non-fatal: the projection starts
// empty and fills in as new events arrive.
func (d *Daemon) newEventStore(stateDir string) error {
	eventStorePath := filepath.Join(stateDir, "events.db")
	eventStore, err := eventstore.NewSQLiteStore(eventStorePath)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "failed to create event store").Build()
	}
	d.eventStore = eventStore
	d.buildProjection = eventstore.NewBuildHistoryProjection(eventStore, 100)
	d.eventEmitter = NewEventEmitter(eventStore, d.buildProjection)
	d.eventEmitter.daemon = d // Wire back reference for hooks (e.g. link verification).

	if rebuildErr := d.buildProjection.Rebuild(context.Background()); rebuildErr != nil {
		slog.Warn("Failed to rebuild build history projection", logfields.Error(rebuildErr))
		// Non-fatal: projection will start empty.
	}
	return nil
}

// collectHTTPInputs builds the per-forge webhook configs and forge clients
// maps consumed by newHTTPServer. Forge clients are sourced from
// d.forgeManager (set by newForgeManager).
func (d *Daemon) collectHTTPInputs() (map[string]*config.WebhookConfig, map[string]forge.Client) {
	webhookConfigs := make(map[string]*config.WebhookConfig)
	for _, forgeCfg := range d.config.Forges {
		if forgeCfg == nil {
			continue
		}
		if forgeCfg.Webhook != nil {
			webhookConfigs[forgeCfg.Name] = forgeCfg.Webhook
		}
	}
	forgeClients := make(map[string]forge.Client)
	if d.forgeManager != nil {
		maps.Copy(forgeClients, d.forgeManager.GetAllForges())
	}
	return webhookConfigs, forgeClients
}

// newHTTPServer initializes d.httpServer with the given webhook configs and
// forge clients. Other wiring (status page, enhanced health, detailed
// metrics, prometheus, triggers, metrics source) is sourced from d.
func (d *Daemon) newHTTPServer(webhookConfigs map[string]*config.WebhookConfig, forgeClients map[string]forge.Client) {
	var detailedMetrics http.HandlerFunc
	if d.metrics != nil {
		detailedMetrics = d.metrics.MetricsHandler
	}
	statusHandlers := handlers.NewStatusPageHandlers(d)
	d.httpServer = httpserver.New(d.config, d, httpserver.Options{
		ForgeClients:          forgeClients,
		WebhookConfigs:        webhookConfigs,
		LiveReloadHub:         d.liveReload,
		EnhancedHealthHandle:  d.EnhancedHealthHandler,
		DetailedMetricsHandle: detailedMetrics,
		PrometheusHandler:     prometheusOptionalHandler(),
		StatusHandle:          statusHandlers.HandleStatusPage,
		Triggers:              d,
		Metrics:               d,
	})
}

// initLinkVerifier initializes d.linkVerifier when link verification is
// enabled. A failure to initialize is logged but non-fatal: the daemon can
// still run without link verification.
func (d *Daemon) initLinkVerifier() {
	cfg := d.config.Daemon.LinkVerification
	lv, err := linkverify.NewVerificationService(cfg)
	if err != nil {
		slog.Warn("Failed to initialize link verification service",
			logfields.Error(err),
			slog.Bool("enabled", false))
		return
	}
	d.linkVerifier = lv
	slog.Info("Link verification service initialized",
		"nats_url", cfg.NATSURL,
		"kv_bucket", cfg.KVBucket)
}

// newDiscoveryRunner initializes d.discoveryRunner with the Phase H extracted
// component. It depends on d.discovery, d.discoveryCache, d.metrics,
// d.stateManager, d.config, and the daemon's discovery callbacks.
func (d *Daemon) newDiscoveryRunner() {
	d.discoveryRunner = NewDiscoveryRunner(DiscoveryRunnerConfig{
		Discovery:      d.discovery,
		DiscoveryCache: d.discoveryCache,
		Metrics:        d.metrics,
		StateManager:   d.stateManager,
		BuildRequester: d.onDiscoveryBuildRequest,
		RepoRemoved:    d.onDiscoveryRepoRemoved,
		Config:         d.config,
	})
}

// newBuildDebouncer initializes d.buildDebouncer with the parsed debounce
// durations and a callback that checks whether the build queue is busy.
//
// The debouncer is passive until components start publishing
// BuildRequested events onto d.orchestrationBus.
func (d *Daemon) newBuildDebouncer(cfg *config.Config) error {
	quietWindow, maxDelay, err := getBuildDebounceDurations(cfg)
	if err != nil {
		return err
	}
	debouncer, err := NewBuildDebouncer(d.orchestrationBus, BuildDebouncerConfig{
		QuietWindow: quietWindow,
		MaxDelay:    maxDelay,
		Metrics:     d.metrics,
		CheckBuildRunning: func() bool {
			if d.buildQueue == nil {
				return false
			}
			return len(d.buildQueue.GetActiveJobs()) > 0
		},
	})
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "failed to create build debouncer").Build()
	}
	d.buildDebouncer = debouncer
	return nil
}

// newRepoUpdater initializes the git client and d.repoUpdater. The git
// client is wired with a remote-HEAD cache rooted at the configured repo
// cache directory; if persistence fails the daemon logs a warning and
// falls back to an in-memory cache.
func (d *Daemon) newRepoUpdater() {
	remoteCache, cacheErr := git.NewRemoteHeadCache(d.config.Daemon.Storage.RepoCacheDir)
	if cacheErr != nil {
		slog.Warn("Failed to initialize remote HEAD cache; disabling persistence", logfields.Error(cacheErr))
		remoteCache, _ = git.NewRemoteHeadCache("")
	}
	gitClient := git.NewClient(d.config.Daemon.Storage.RepoCacheDir).WithRemoteHeadCache(remoteCache)
	d.repoUpdater = NewRepoUpdater(d.orchestrationBus, gitClient, remoteCache, d.currentReposForOrchestratedBuild)
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
			return 0, 0, derrors.WrapError(err, derrors.CategoryConfig, "failed to parse daemon.build_debounce.quiet_window").Build()
		}
		quietWindow = parsed
	}
	if v := strings.TrimSpace(cfg.Daemon.BuildDebounce.MaxDelay); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return 0, 0, derrors.WrapError(err, derrors.CategoryConfig, "failed to parse daemon.build_debounce.max_delay").Build()
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
		return derrors.NewError(derrors.CategoryValidation, "daemon is not in stopped state: "+d.GetStatus()).Build()
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
	runCtx, runCancel := context.WithCancel(ctx)
	d.runCancel = runCancel
	d.workers.Reset()

	// Start HTTP servers
	if err := d.httpServer.Start(runCtx); err != nil {
		d.status.Store(StatusError)
		d.runCancel = nil
		runCancel()
		d.mu.Unlock()
		return derrors.WrapError(err, derrors.CategoryNetwork, "failed to start HTTP server").Build()
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
		return derrors.WrapError(err, derrors.CategoryInternal, "failed to schedule daemon jobs").Build()
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
			workCtx, cancel := d.stopAwareContext(ctx)
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

	if err := d.workers.StopAndWait(ctx); err != nil {
		slog.Warn("Timed out waiting for daemon workers to stop", logfields.Error(err))
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
	return int(d.queueLength.Load())
}

// GetStartTime returns the daemon start time.
func (d *Daemon) GetStartTime() time.Time {
	return d.startTime
}

// Compile-time assertions that *Daemon implements the optional httpserver
// runtime surfaces supplied to httpserver.New(cfg, daemon, opts).
var (
	_ httpserver.Status        = (*Daemon)(nil)
	_ httpserver.Triggers      = (*Daemon)(nil)
	_ httpserver.MetricsSource = (*Daemon)(nil)
)

// BuildEventEmitter is implemented by *EventEmitter; see event_emitter.go.
// Daemon no longer claims to implement it (the methods were pure
// delegates to d.eventEmitter). Wire BuildQueue with d.eventEmitter
// directly.
