package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
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
//
// The struct is a thin façade over six per-concern sub-structs.
// The sub-structs own the daemon's dependencies; the Daemon itself
// holds only the configuration, the lifecycle state, and the two
// shared atomic counters. The goal is that adding a new dependency
// touches one sub-struct, not the top-level struct, so that the
// public surface stays small and the concern groups stay cohesive.
//
// The sub-structs are:
//   - lifecycle:  status, run coordination, scheduled-job bookkeeping
//   - state:      persistence (state manager, event store, projection, emitter)
//   - build:      build pipeline (BuildService, queue, debouncer, repo updater)
//   - discover:   discovery pipeline (forge manager, service, cache, runner)
//   - serve:      runtime services (HTTP server, scheduler, event bus, metrics)
//   - optional:   opt-in features (livereload, link verifier, outbound dispatcher)
//
// The shared counters `activeJobs` and `queueLength` stay on the
// façade because they are read from multiple sub-systems and would
// have to be back-referenced from the sub-structs otherwise.
type Daemon struct {
	config         *config.Config
	configFilePath string
	activeJobs     int32
	queueLength    atomic.Int32

	lifecycle lifecycleStack
	state     *stateStack
	build     *buildStack
	discover  *discoveryStack
	serve     *serveStack
	optional  *optionalStack
}

// NewDaemon creates a new daemon instance. It is a thin wrapper
// around NewDaemonWithConfigFile with no config file to watch.
func NewDaemon(cfg *config.Config) (*Daemon, error) {
	return NewDaemonWithConfigFile(cfg, "")
}

// NewDaemonWithConfigFile creates a new daemon instance with config file watching.
//
// The constructor is intentionally a thin recipe over per-concern init
// methods. Each init method owns one subsystem and either returns an
// error or is safe to ignore. Reading the recipe top-to-bottom is the
// fastest way to see what a fully-initialized daemon contains; reading
// any single init method is the fastest way to understand one concern.
func NewDaemonWithConfigFile(cfg *config.Config, configFilePath string) (*Daemon, error) {
	if cfg == nil {
		return nil, errors.New("configuration is required")
	}
	if cfg.Daemon == nil {
		return nil, errors.New("daemon configuration is required")
	}

	d := &Daemon{
		config:         cfg,
		configFilePath: configFilePath,
		lifecycle:      lifecycleStack{},
		state:          &stateStack{},
		build:          &buildStack{},
		discover:       &discoveryStack{},
		serve:          &serveStack{},
		optional:       &optionalStack{},
	}

	d.initLifecycle()
	d.initServeStack()
	d.initDiscoveryStack()
	if err := d.initForges(cfg); err != nil {
		return nil, err
	}
	d.initBuildStack(cfg)
	if ragCfg := ragabastConfigOrNil(cfg); ragCfg != nil {
		if err := d.initOutboundDispatcher(ragCfg); err != nil {
			return nil, err
		}
	}
	if err := d.initScheduler(); err != nil {
		return nil, err
	}
	if err := d.initStateStack(cfg); err != nil {
		return nil, err
	}
	if cfg.Build.LiveReload {
		d.initLiveReload()
	}
	d.initHTTPServer(cfg)
	if cfg.Daemon.LinkVerification != nil && cfg.Daemon.LinkVerification.Enabled {
		d.initLinkVerifier(cfg)
	}
	// Cross-wiring: the build queue emits events through the event
	// emitter, but the emitter is constructed by initStateStack which
	// runs after initBuildStack. The recipe is the right place to
	// express that ordering dependency.
	d.build.buildQueue.SetEventEmitter(d.state.eventEmitter)

	d.initDiscoveryRunner(cfg)
	if err := d.initBuildDebouncer(cfg); err != nil {
		return nil, err
	}
	d.initRepoUpdater(cfg)

	return d, nil
}

// initLifecycle prepares the lifecycle sub-struct: the run-coordination
// channels and the initial status. Every other init method can rely
// on lifecycle.status being usable.
func (d *Daemon) initLifecycle() {
	d.lifecycle.stopChan = make(chan struct{})
	d.lifecycle.status.Store(StatusStopped)
}

// initServeStack prepares the serve sub-struct: the metrics collector
// and the in-process orchestration event bus. The HTTP server and the
// cron-like scheduler are constructed by later init steps that depend
// on these.
func (d *Daemon) initServeStack() {
	d.serve.metrics = NewMetricsCollector()
	d.serve.orchestrationBus = events.NewBus()
}

// initDiscoveryStack prepares the discovery sub-struct's always-on
// components: the in-memory cache that backs the status page and
// HTTP responses. The forge manager and discovery service are
// constructed by initForges once the configured forges are
// available.
func (d *Daemon) initDiscoveryStack() {
	d.discover.discoveryCache = NewDiscoveryCache()
}

// initForges builds the forge manager from cfg.Forges and the discovery
// service that fronts it. Returns an error if any configured forge
// fails to construct.
func (d *Daemon) initForges(cfg *config.Config) error {
	forgeManager := forge.NewForgeManager()
	for _, forgeConfig := range cfg.Forges {
		client, err := forge.NewForgeClient(forgeConfig)
		if err != nil {
			return fmt.Errorf("failed to create forge client %s: %w", forgeConfig.Name, err)
		}
		forgeManager.AddForge(forgeConfig, client)
	}
	d.discover.forgeManager = forgeManager
	d.discover.discovery = forge.NewDiscoveryService(forgeManager, cfg.Filtering)
	return nil
}

// initBuildStack builds the canonical BuildService and the build queue
// that dispatches jobs to the daemon itself (which satisfies
// queue.Builder). The queue's retry policy is configured from
// cfg.Build.
func (d *Daemon) initBuildStack(cfg *config.Config) {
	// The daemon itself implements queue.Builder (see (*Daemon).Build),
	// so no adapter type is needed.
	d.build.buildSvc = build.NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			// Use persistent workspace for incremental builds (repo_cache_dir/working).
			return workspace.NewPersistentManager(cfg.Daemon.Storage.RepoCacheDir, "working")
		})

	d.build.buildQueue = queue.NewBuildQueue(cfg.Daemon.Sync.QueueSize, cfg.Daemon.Sync.ConcurrentBuilds, d)
	// Configure retry policy from build config. Recorder injection is
	// handled elsewhere if added later.
	d.build.buildQueue.ConfigureRetry(cfg.Build)
}

// initScheduler constructs the cron-like scheduler that drives the
// daemon's periodic work (sync tick, status update, prom sync).
func (d *Daemon) initScheduler() error {
	scheduler, err := NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}
	d.serve.scheduler = scheduler
	return nil
}

// initStateStack builds the state manager, event store, build history
// projection, and event emitter. All four are tightly coupled: the
// state manager is the typed source of truth, the event store is the
// append-only log, the projection is the read-side view, and the
// emitter translates daemon events into the store.
func (d *Daemon) initStateStack(cfg *config.Config) error {
	stateDir := cfg.Daemon.Storage.RepoCacheDir
	if stateDir == "" {
		stateDir = "./daemon-data" // Default data directory
	}

	stateServiceResult := state.NewService(stateDir)
	if stateServiceResult.IsErr() {
		return fmt.Errorf("failed to create state service: %w", stateServiceResult.UnwrapErr())
	}
	d.state.stateManager = state.NewServiceAdapter(stateServiceResult.Unwrap())

	eventStorePath := filepath.Join(stateDir, "events.db")
	eventStore, err := eventstore.NewSQLiteStore(eventStorePath)
	if err != nil {
		return fmt.Errorf("failed to create event store: %w", err)
	}
	d.state.eventStore = eventStore
	d.state.buildProjection = eventstore.NewBuildHistoryProjection(eventStore, 100)
	d.state.eventEmitter = NewEventEmitter(eventStore, d.state.buildProjection)
	d.state.eventEmitter.daemon = d // Wire back reference for hooks

	// Rebuild projection from existing events. Non-fatal: a corrupt log
	// leaves the projection empty rather than failing daemon startup.
	if rebuildErr := d.state.buildProjection.Rebuild(context.Background()); rebuildErr != nil {
		slog.Warn("Failed to rebuild build history projection", logfields.Error(rebuildErr))
	}
	return nil
}

// initLiveReload constructs the SSE-based live reload hub. Only invoked
// when cfg.Build.LiveReload is true. Safe to call on a nil metrics
// collector — the hub falls back to a no-op recorder.
func (d *Daemon) initLiveReload() {
	d.optional.liveReload = NewLiveReloadHub(d.serve.metrics)
	slog.Info("LiveReload hub initialized")
}

// initHTTPServer builds the HTTP server with its full wiring (forge
// clients, webhooks, live reload hub, health, metrics, status page).
// The server is started by (*Daemon).Start; this method only
// constructs it.
func (d *Daemon) initHTTPServer(cfg *config.Config) {
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
	if d.discover.forgeManager != nil {
		maps.Copy(forgeClients, d.discover.forgeManager.GetAllForges())
	}

	var detailedMetrics http.HandlerFunc
	if d.serve.metrics != nil {
		detailedMetrics = d.serve.metrics.MetricsHandler
	}

	statusHandlers := handlers.NewStatusPageHandlers(d)
	d.serve.httpServer = httpserver.New(cfg, d, httpserver.Options{
		ForgeClients:          forgeClients,
		WebhookConfigs:        webhookConfigs,
		LiveReloadHub:         d.optional.liveReload,
		EnhancedHealthHandle:  d.EnhancedHealthHandler,
		DetailedMetricsHandle: detailedMetrics,
		PrometheusHandler:     prometheusOptionalHandler(),
		StatusHandle:          statusHandlers.HandleStatusPage,
	})
}

// initLinkVerifier attempts to construct the link verification service.
// It is opt-in via cfg.Daemon.LinkVerification.Enabled and tolerates
// construction failure (e.g. NATS unreachable) by leaving linkVerifier
// nil and logging a warning. The optional path is the only one that
// touches external services during construction.
func (d *Daemon) initLinkVerifier(cfg *config.Config) {
	linkVerifier, linkVerifierErr := linkverify.NewVerificationService(cfg.Daemon.LinkVerification)
	if linkVerifierErr != nil {
		slog.Warn("Failed to initialize link verification service",
			logfields.Error(linkVerifierErr),
			slog.Bool("enabled", false))
		return
	}
	d.optional.linkVerifier = linkVerifier
	slog.Info("Link verification service initialized",
		"nats_url", cfg.Daemon.LinkVerification.NATSURL,
		"kv_bucket", cfg.Daemon.LinkVerification.KVBucket)
}

// initDiscoveryRunner builds the runner that owns the daemon's
// discovery flow (ADR-021). It depends on the discovery service, the
// forge manager, the cache, the metrics collector, the state manager,
// the live reload hub, and the daemon's discovery callbacks.
func (d *Daemon) initDiscoveryRunner(cfg *config.Config) {
	d.discover.discoveryRunner = NewDiscoveryRunner(DiscoveryRunnerConfig{
		Discovery:      d.discover.discovery,
		ForgeManager:   d.discover.forgeManager,
		DiscoveryCache: d.discover.discoveryCache,
		Metrics:        d.serve.metrics,
		StateManager:   d.state.stateManager,
		BuildRequester: d.onDiscoveryBuildRequest,
		RepoRemoved:    d.onDiscoveryRepoRemoved,
		LiveReload:     d.optional.liveReload,
		Config:         cfg,
	})
}

// initBuildDebouncer constructs the build debouncer (ADR-021 Phase 2).
// Returns an error if the configured quiet window or max delay is not
// a valid Go duration string. The debouncer is passive until
// components start publishing BuildRequested events.
func (d *Daemon) initBuildDebouncer(cfg *config.Config) error {
	quietWindow, maxDelay, err := getBuildDebounceDurations(cfg)
	if err != nil {
		return err
	}
	debouncer, err := NewBuildDebouncer(d.serve.orchestrationBus, BuildDebouncerConfig{
		QuietWindow: quietWindow,
		MaxDelay:    maxDelay,
		Metrics:     d.serve.metrics,
		CheckBuildRunning: func() bool {
			if d.build.buildQueue == nil {
				return false
			}
			return len(d.build.buildQueue.GetActiveJobs()) > 0
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create build debouncer: %w", err)
	}
	d.build.buildDebouncer = debouncer
	return nil
}

// initRepoUpdater constructs the git-backed repo updater that polls
// remote HEADs and publishes RepoUpdated events on the orchestration
// bus. If the persistent remote-head cache cannot be created
// (e.g. permission error on the storage dir) the cache is degraded to
// in-memory and a warning is logged — repo-updates keep working, just
// without persistence across restarts.
func (d *Daemon) initRepoUpdater(cfg *config.Config) {
	remoteCache, cacheErr := git.NewRemoteHeadCache(cfg.Daemon.Storage.RepoCacheDir)
	if cacheErr != nil {
		slog.Warn("Failed to initialize remote HEAD cache; disabling persistence", logfields.Error(cacheErr))
		remoteCache, _ = git.NewRemoteHeadCache("")
	}
	gitClient := git.NewClient(cfg.Daemon.Storage.RepoCacheDir).WithRemoteHeadCache(remoteCache)
	d.build.repoUpdater = NewRepoUpdater(d.serve.orchestrationBus, gitClient, remoteCache, d.currentReposForOrchestratedBuild)
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

// ragabastConfigOrNil returns the ragabast outbound config when both the
// daemon.outbound.ragabast block is present AND explicitly enabled. Returns
// nil otherwise (caller treats nil as "feature disabled — do nothing").
//
// Centralizing this check here keeps the construction site free of repeated
// nil-pointer guards and makes the opt-in semantics testable.
func ragabastConfigOrNil(cfg *config.Config) *config.RagabastConfig {
	if cfg == nil || cfg.Daemon == nil || cfg.Daemon.Outbound == nil {
		return nil
	}
	r := cfg.Daemon.Outbound.Ragabast
	if r == nil || !r.Enabled {
		return nil
	}
	return r
}

// initOutboundDispatcher constructs and attaches the ragabast outbound
// dispatcher to the daemon. Extracted from NewDaemon to keep the construction
// site readable and to avoid a nestif warning.
func (d *Daemon) initOutboundDispatcher(ragCfg *config.RagabastConfig) error {
	token := ""
	if ragCfg.AuthTokenEnv != "" {
		token = os.Getenv(ragCfg.AuthTokenEnv)
	}
	timeout := 10 * time.Second
	if t := strings.TrimSpace(ragCfg.Timeout); t != "" {
		parsed, err := time.ParseDuration(t)
		if err != nil {
			return fmt.Errorf("invalid daemon.outbound.ragabast.timeout: %w", err)
		}
		timeout = parsed
	}
	dispatcher, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL:          ragCfg.BaseURL,
		IngestURL:        ragCfg.IngestURL,
		Token:            token,
		Workers:          ragCfg.Workers,
		QueueSize:        ragCfg.QueueSize,
		Timeout:          timeout,
		PreflightBaseURL: ragCfg.PreflightBaseURL,
		PreflightEnabled: ragCfg.PreflightEnabled,
	})
	if err != nil {
		return fmt.Errorf("failed to construct outbound dispatcher: %w", err)
	}
	d.optional.outboundDispatcher = dispatcher
	slog.Info("Outbound dispatcher initialized (ragabast)",
		slog.String("ragabast_base_url", ragCfg.BaseURL),
		slog.String("ingest_url", ragCfg.IngestURL),
		slog.Bool("auth_enabled", token != ""))
	return nil
}

// defaultDaemonInstance is used by optional Prometheus integration to pull metrics
// into the Prometheus registry when the build tag is enabled.
var defaultDaemonInstance *Daemon

// Start starts the daemon and all its components.
func (d *Daemon) Start(ctx context.Context) error {
	d.lifecycle.mu.Lock()
	if d.GetStatus() != StatusStopped {
		d.lifecycle.mu.Unlock()
		return fmt.Errorf("daemon is not in stopped state: %s", d.GetStatus())
	}

	d.lifecycle.status.Store(StatusStarting)
	d.lifecycle.startTime = time.Now()

	// Initialize metrics
	d.serve.metrics.IncrementCounter("daemon_starts")
	d.serve.metrics.SetGauge("daemon_status", int64(1)) // 1 = starting

	// Set global reference for metrics bridge (prometheus build only uses it).
	defaultDaemonInstance = d
	slog.Info("Starting DocBuilder daemon", slog.String("version", "2.0"))

	// Load persistent state
	if err := d.state.stateManager.Load(); err != nil {
		slog.Warn("Failed to load state", "error", err)
	}

	// Create a derived run context that is canceled on daemon shutdown.
	runCtx, runCancel := context.WithCancel(ctx)
	d.lifecycle.runCancel = runCancel
	d.lifecycle.workers.Reset()

	// Start HTTP servers
	if err := d.serve.httpServer.Start(runCtx); err != nil {
		d.lifecycle.status.Store(StatusError)
		d.lifecycle.runCancel = nil
		runCancel()
		d.lifecycle.mu.Unlock()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start build queue processing
	d.build.buildQueue.Start(runCtx)

	// Start outbound dispatcher (opt-in). When nil, this is a no-op.
	if d.optional.outboundDispatcher != nil {
		d.optional.outboundDispatcher.Start(runCtx)
	}

	d.startWorkers(runCtx)

	// Schedule periodic daemon work (cron/duration jobs) before starting the scheduler.
	if err := d.schedulePeriodicJobs(runCtx); err != nil {
		d.lifecycle.status.Store(StatusError)
		if d.lifecycle.runCancel != nil {
			d.lifecycle.runCancel()
			d.lifecycle.runCancel = nil
		}
		d.lifecycle.mu.Unlock()
		return fmt.Errorf("failed to schedule daemon jobs: %w", err)
	}

	// Start scheduler
	d.serve.scheduler.Start(ctx)

	d.lifecycle.status.Store(StatusRunning)
	d.serve.metrics.SetGauge("daemon_status", int64(2)) // 2 = running
	d.serve.metrics.IncrementCounter("daemon_successful_starts")

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
	d.lifecycle.mu.Unlock()

	// Run main daemon loop (blocks until stopped)
	d.mainLoop(runCtx)

	// When mainLoop exits, we're stopping
	d.lifecycle.status.Store(StatusStopping)
	slog.Info("Main loop exited, daemon stopping")

	return nil
}

func (d *Daemon) schedulePeriodicJobs(ctx context.Context) error {
	if d.serve.scheduler == nil {
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

	syncJobID, err := d.serve.scheduler.ScheduleCron("daemon-sync", expr, func() {
		d.runScheduledSyncTick(ctx, expr)
	})
	if err != nil {
		return err
	}
	d.lifecycle.syncJobID = syncJobID

	statusJobID, err := d.serve.scheduler.ScheduleEvery("daemon-status", 30*time.Second, func() {
		if d.GetStatus() != StatusRunning {
			return
		}
		d.updateStatus()
	})
	if err != nil {
		return err
	}
	d.lifecycle.statusJobID = statusJobID

	// Prometheus counter bridge sync (used by /metrics handler). This replaces the
	// previous global goroutine+sleep loop so the daemon owns the periodic work.
	promJobID, err := d.serve.scheduler.ScheduleEvery("daemon-prom-sync", 5*time.Second, func() {
		if d.GetStatus() != StatusRunning {
			return
		}
		updateDaemonPromMetrics(d)
	})
	if err != nil {
		return err
	}
	d.lifecycle.promJobID = promJobID

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
		if d.discover.discoveryRunner == nil {
			slog.Warn("Skipping scheduled discovery: discovery runner not initialized")
		} else {
			workCtx, cancel := d.stopAwareContext(ctx)
			defer cancel()
			d.discover.discoveryRunner.SafeRun(workCtx, func() bool { return d.GetStatus() == StatusRunning })
		}
	}

	// For explicit repositories, trigger a build to check for updates.
	if len(d.config.Repositories) > 0 {
		if d.serve.orchestrationBus == nil {
			slog.Warn("Skipping scheduled build: orchestration bus not initialized")
		} else {
			d.triggerScheduledBuildForExplicitRepos(ctx)
		}
	}
}

// Stop gracefully shuts down the daemon.
func (d *Daemon) Stop(ctx context.Context) error {
	d.lifecycle.mu.Lock()
	currentStatus := d.GetStatus()
	if currentStatus == StatusStopped || currentStatus == StatusStopping {
		d.lifecycle.mu.Unlock()
		return nil
	}

	d.lifecycle.status.Store(StatusStopping)
	slog.Info("Stopping DocBuilder daemon")

	// Snapshot pointers so we can stop without holding the daemon mutex.
	runCancel := d.lifecycle.runCancel
	d.lifecycle.runCancel = nil
	stopChan := d.lifecycle.stopChan
	bus := d.serve.orchestrationBus
	scheduler := d.serve.scheduler
	buildQueue := d.build.buildQueue
	httpServer := d.serve.httpServer
	liveReload := d.optional.liveReload
	linkVerifier := d.optional.linkVerifier
	stateManager := d.state.stateManager
	eventStore := d.state.eventStore
	outboundDispatcher := d.optional.outboundDispatcher
	d.lifecycle.mu.Unlock()

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

	// Stop the outbound dispatcher last among workers — it has its own
	// drain semantics (workers process remaining queue items, then exit).
	// Canceling runCtx above cancels in-flight POSTs; Stop then drains.
	if outboundDispatcher != nil {
		outboundDispatcher.Stop()
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

	if err := d.lifecycle.workers.StopAndWait(ctx); err != nil {
		slog.Warn("Timed out waiting for daemon workers to stop", logfields.Error(err))
	}

	d.lifecycle.mu.Lock()
	d.lifecycle.status.Store(StatusStopped)
	d.lifecycle.mu.Unlock()

	uptime := time.Since(d.lifecycle.startTime)
	slog.Info("DocBuilder daemon stopped", slog.Duration("uptime", uptime))

	return nil
}

// GetStatus returns the current daemon status.
func (d *Daemon) GetStatus() Status {
	status, ok := d.lifecycle.status.Load().(Status)
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
	return d.lifecycle.startTime
}

// Compile-time check that Daemon implements queue.BuildEventEmitter.
var _ queue.BuildEventEmitter = (*Daemon)(nil)

// Compile-time check that Daemon implements queue.Builder.
var _ queue.Builder = (*Daemon)(nil)

// Build is the queue.Builder entry point. It serializes against other
// build jobs (so concurrent workers don't clobber shared staging), then
// translates the job to a BuildRequest and runs it through the canonical
// BuildService.
func (d *Daemon) Build(ctx context.Context, job *queue.BuildJob) (*models.BuildReport, error) {
	if job == nil {
		return nil, errors.New("build job is nil")
	}
	d.build.buildMu.Lock()
	defer d.build.buildMu.Unlock()

	req, err := d.jobToBuildRequest(job)
	if err != nil {
		return nil, err
	}
	result, err := d.build.buildSvc.Run(ctx, req)
	if err != nil {
		return nil, err
	}
	return result.Report, nil
}

// jobToBuildRequest converts a daemon BuildJob into the canonical
// BuildRequest. It applies the per-job repository override (for
// orchestration flows), the BaseDirectory+Directory resolution (so
// daemon-mode jobs honor cfg.Output.BaseDirectory), and the daemon's
// per-build wiring:
//
//   - SkipState is set to the daemon's state manager so the canonical
//     BuildService can run skip-evaluation when build.skip_if_unchanged
//     is configured (previously wired via the HugoGeneratorFactory closure).
//   - OnDocumentReady is set to the outbound dispatcher's Enqueue when
//     ragabast ingest is enabled, so built documents are pushed to the
//     external indexer as they are written.
//
// Both wiring points were lost when the BuildServiceAdapter and its
// HugoGeneratorFactory closure were deleted in Phase 4 of the
// build-service-unification refactor. The wiring is restored here as
// explicit fields on BuildRequest. nil/empty values keep the CLI and
// the no-ragabast default case free of overhead.
func (d *Daemon) jobToBuildRequest(job *queue.BuildJob) (build.BuildRequest, error) {
	var cfg *config.Config
	if job.TypedMeta != nil && job.TypedMeta.V2Config != nil {
		cfg = job.TypedMeta.V2Config
	}
	if cfg == nil {
		return build.BuildRequest{}, errors.New("build job has no configuration")
	}

	// If the job carries an explicit repository set, prefer it over
	// cfg.Repositories. This enables orchestration flows (ADR-021) to
	// enqueue canonical full-site builds in forge mode where
	// cfg.Repositories may be empty.
	if job.TypedMeta != nil && len(job.TypedMeta.Repositories) > 0 {
		cfgCopy := *cfg
		cfgCopy.Repositories = job.TypedMeta.Repositories
		if len(job.TypedMeta.RepoSnapshot) > 0 {
			for i := range cfgCopy.Repositories {
				repo := &cfgCopy.Repositories[i]
				if sha, ok := job.TypedMeta.RepoSnapshot[repo.URL]; ok && sha != "" {
					repo.PinnedCommit = sha
				}
			}
		}
		cfg = &cfgCopy
	}

	// *state.ServiceAdapter implements validation.SkipStateAccess (all
	// seven getters/setters that BuildService.Run's evaluateSkip path
	// calls). See internal/state/service_adapter.go. The state and
	// optional sub-structs are always populated by the constructor
	// and the daemonForTest helper, so no nil-check is needed here.
	skipState := d.state.stateManager
	var onDocReady func([]byte, string)
	if d.optional.outboundDispatcher != nil {
		onDocReady = d.optional.outboundDispatcher.Enqueue
	}

	return build.BuildRequest{
		Config:          cfg,
		OutputDir:       resolveOutputDir(cfg),
		Incremental:     true, // Daemon mode uses incremental updates to leverage remote HEAD cache
		SkipState:       skipState,
		OnDocumentReady: onDocReady,
		Options: build.BuildOptions{
			SkipIfUnchanged: cfg.Build.SkipIfUnchanged,
		},
	}, nil
}

// resolveOutputDir applies the BaseDirectory+Directory rule used by
// daemon-mode builds: relative directories are joined onto
// BaseDirectory; BaseDirectory is ignored when Directory is absolute.
func resolveOutputDir(cfg *config.Config) string {
	outDir := cfg.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}
	if cfg.Output.BaseDirectory != "" && !filepath.IsAbs(outDir) {
		outDir = filepath.Join(cfg.Output.BaseDirectory, outDir)
	}
	return outDir
}
