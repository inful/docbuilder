package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
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
	// Provide injected dependencies so scheduler can enqueue jobs without a daemon back-reference.
	daemon.scheduler.SetEnqueuer(daemon.buildQueue)
	daemon.scheduler.SetMetaFactory(func() *BuildJobMetadata {
		return &BuildJobMetadata{
			V2Config:      daemon.config,
			StateManager:  daemon.stateManager,
			LiveReloadHub: daemon.liveReload,
		}
	})

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
		linkVerifier, err := linkverify.NewVerificationService(cfg.Daemon.LinkVerification)
		if err != nil {
			slog.Warn("Failed to initialize link verification service",
				logfields.Error(err),
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

// Compile-time check that Daemon implements BuildEventEmitter.
var _ BuildEventEmitter = (*Daemon)(nil)
