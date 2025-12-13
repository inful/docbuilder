package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/state"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// Status represents the current state of the daemon
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusError    Status = "error"
)

// Daemon represents the main daemon service
type Daemon struct {
	config         *config.Config
	configFilePath string
	status         atomic.Value // DaemonStatus
	startTime      time.Time
	stopChan       chan struct{}
	mu             sync.RWMutex

	// Core components
	forgeManager   *forge.Manager
	discovery      *forge.DiscoveryService
	versionService *versioning.VersionService
	configWatcher  *ConfigWatcher
	metrics        *MetricsCollector
	httpServer     *HTTPServer
	scheduler      *Scheduler
	buildQueue     *BuildQueue
	stateManager   state.DaemonStateManager
	liveReload     *LiveReloadHub

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

	// Build status tracker for preview mode (optional, used by local preview)
	buildStatus interface{ getStatus() (bool, error, bool) }
}

// NewDaemon creates a new daemon instance
// NewDaemon creates a new daemon instance
func NewDaemon(cfg *config.Config) (*Daemon, error) {
	return NewDaemonWithConfigFile(cfg, "")
}

// NewDaemonWithConfigFile creates a new daemon instance with config file watching
func NewDaemonWithConfigFile(cfg *config.Config, configFilePath string) (*Daemon, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	if cfg.Daemon == nil {
		return nil, fmt.Errorf("daemon configuration is required")
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
		WithHugoGeneratorFactory(func(cfg any, outputDir string) build.HugoGenerator {
			// Type assert cfg to *config.Config
			configTyped, ok := cfg.(*config.Config)
			if !ok {
				slog.Error("Invalid config type passed to Hugo generator factory")
				return nil
			}
			return hugo.NewGenerator(configTyped, outputDir)
		})
	buildAdapter := NewBuildServiceAdapter(buildService)

	// Initialize build queue with the canonical builder
	daemon.buildQueue = NewBuildQueue(cfg.Daemon.Sync.QueueSize, cfg.Daemon.Sync.ConcurrentBuilds, buildAdapter)
	// Configure retry policy from build config (recorder injection handled elsewhere if added later)
	daemon.buildQueue.ConfigureRetry(cfg.Build)

	// Initialize scheduler (after build queue)
	daemon.scheduler = NewScheduler(daemon.buildQueue)
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

	// Rebuild projection from existing events
	if err := daemon.buildProjection.Rebuild(context.Background()); err != nil {
		slog.Warn("Failed to rebuild build history projection", logfields.Error(err))
		// Non-fatal: projection will start empty
	}

	// Initialize version service
	// Create a git client for version management (use same workspace as daemon)
	gitClient := git.NewClient(stateDir)
	versionManager := versioning.NewVersionManager(gitClient)

	// For now, use default version config - this can be made configurable later
	versionConfig := &versioning.VersionConfig{
		Strategy:    versioning.StrategyDefaultOnly,
		MaxVersions: 10,
	}
	daemon.versionService = versioning.NewVersionService(versionManager, versionConfig)

	// Initialize config watcher if config file path is provided
	if configFilePath != "" {
		var err error
		daemon.configWatcher, err = NewConfigWatcher(configFilePath, daemon)
		if err != nil {
			return nil, fmt.Errorf("failed to create config watcher: %w", err)
		}
	}

	// Initialize livereload hub (opt-in)
	if cfg.Build.LiveReload {
		daemon.liveReload = NewLiveReloadHub(daemon.metrics)
		slog.Info("LiveReload hub initialized")
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

// Start starts the daemon and all its components
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

	// Start config watcher if available
	if d.configWatcher != nil {
		if err := d.configWatcher.Start(ctx); err != nil {
			slog.Error("Failed to start config watcher", "error", err)
		} else {
			slog.Info("Config watcher started")
		}
	}

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
	if d.config.Build.WorkspaceDir != "" {
		wsPredict = d.config.Build.WorkspaceDir + " (configured)"
	} else if strategy == config.CloneStrategyFresh {
		wsPredict = filepath.Join(outDir, "_workspace") + " (ephemeral)"
	} else if repoCache != "" {
		wsPredict = filepath.Join(repoCache, "working") + " (persistent via repo_cache_dir)"
	} else {
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

// Stop gracefully shuts down the daemon
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
	if d.configWatcher != nil {
		if err := d.configWatcher.Stop(ctx); err != nil {
			slog.Error("Failed to stop config watcher", "error", err)
		}
	}

	if d.scheduler != nil {
		d.scheduler.Stop(ctx)
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

// GetStatus returns the current daemon status
func (d *Daemon) GetStatus() Status {
	status, ok := d.status.Load().(Status)
	if !ok {
		return StatusError
	}
	return status
}

// GetActiveJobs returns the number of active build jobs
func (d *Daemon) GetActiveJobs() int {
	return int(atomic.LoadInt32(&d.activeJobs))
}

// GetQueueLength returns the current build queue length
func (d *Daemon) GetQueueLength() int {
	return int(atomic.LoadInt32(&d.queueLength))
}

// GetStartTime returns the daemon start time
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

// EmitBuildReport implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildReport(ctx context.Context, buildID string, report *hugo.BuildReport) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildReport(ctx, buildID, report)
}

// Compile-time check that Daemon implements BuildEventEmitter
var _ BuildEventEmitter = (*Daemon)(nil)

// TriggerDiscovery manually triggers repository discovery
func (d *Daemon) TriggerDiscovery() string {
	return d.discoveryRunner.TriggerManual(d.GetStatus, &d.activeJobs)
}

// TriggerBuild manually triggers a site build
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

	atomic.AddInt32(&d.queueLength, 1)
	return jobID
}

// mainLoop runs the main daemon processing loop
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
			go d.discoveryRunner.SafeRun(d.GetStatus)
		case <-discoveryTicker.C:
			slog.Info("Scheduled discovery tick", slog.Duration("interval", discoveryInterval))
			go d.discoveryRunner.SafeRun(d.GetStatus)
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
	if strings.HasPrefix(expr, "@every ") {
		rem := strings.TrimSpace(strings.TrimPrefix(expr, "@every "))
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
		if strings.HasPrefix(parts[0], "*/") {
			val := strings.TrimPrefix(parts[0], "*/")
			if n, err := strconv.Atoi(val); err == nil && n > 0 && n < 60 {
				return time.Duration(n) * time.Minute, true
			}
		}
	}
	return 0, false
}

// updateStatus updates runtime status and metrics
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

// GetConfig returns the current daemon configuration
func (d *Daemon) GetConfig() *config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// ReloadConfig reloads the daemon configuration and restarts affected services
func (d *Daemon) ReloadConfig(ctx context.Context, newConfig *config.Config) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	slog.Info("Reloading daemon configuration")

	// Store old config for rollback if needed
	oldConfig := d.config

	// Update daemon config
	d.config = newConfig

	// Restart components that depend on configuration
	if err := d.reloadForgeManager(ctx, oldConfig, newConfig); err != nil {
		d.config = oldConfig // Rollback
		return fmt.Errorf("failed to reload forge manager: %w", err)
	}

	if err := d.reloadVersionService(ctx, oldConfig, newConfig); err != nil {
		d.config = oldConfig // Rollback
		return fmt.Errorf("failed to reload version service: %w", err)
	}

	slog.Info("Configuration reloaded successfully")

	// Trigger a rebuild to apply the new configuration
	// This is done outside the lock to avoid deadlock, so we return immediately
	// and let the goroutine handle the build trigger
	go func() {
		// Use a short delay to ensure config is fully applied
		time.Sleep(500 * time.Millisecond)

		jobID := d.TriggerBuild()
		if jobID != "" {
			slog.Info("Triggered rebuild after config reload", "job_id", jobID)
		} else {
			slog.Warn("Failed to trigger rebuild after config reload - daemon may not be running")
		}
	}()

	return nil
}

// reloadForgeManager updates the forge manager with new forge configurations
func (d *Daemon) reloadForgeManager(_ context.Context, _, newConfig *config.Config) error {
	// Create new forge manager
	newForgeManager := forge.NewForgeManager()
	for _, forgeConfig := range newConfig.Forges {
		client, err := forge.NewForgeClient(forgeConfig)
		if err != nil {
			return fmt.Errorf("failed to create forge client %s: %w", forgeConfig.Name, err)
		}
		newForgeManager.AddForge(forgeConfig, client)
	}

	// Replace forge manager
	d.forgeManager = newForgeManager

	// Update discovery service
	d.discovery = forge.NewDiscoveryService(newForgeManager, newConfig.Filtering)

	// Update discovery runner with new services
	d.discoveryRunner.UpdateForgeManager(newForgeManager)
	d.discoveryRunner.UpdateDiscoveryService(d.discovery)
	d.discoveryRunner.UpdateConfig(newConfig)

	slog.Info("Forge manager reloaded", slog.Int("forge_count", len(newConfig.Forges)))
	return nil
}

// reloadVersionService updates the version service with new versioning configuration
// nolint:unparam // This method currently never returns an error.
func (d *Daemon) reloadVersionService(_ context.Context, _, newConfig *config.Config) error {
	// Create new version configuration
	versionConfig := &versioning.VersionConfig{
		Strategy:    versioning.StrategyDefaultOnly,
		MaxVersions: 10,
	}

	// Update versioning config if present in new config
	if newConfig.Versioning != nil {
		// Convert V2Config versioning to internal versioning config
		versionConfig = versioning.GetVersioningConfig(newConfig)
	}

	// Create new version service
	stateDir := newConfig.Daemon.Storage.RepoCacheDir
	if stateDir == "" {
		stateDir = "./daemon-data"
	}
	gitClient := git.NewClient(stateDir)
	versionManager := versioning.NewVersionManager(gitClient)

	d.versionService = versioning.NewVersionService(versionManager, versionConfig)

	slog.Info("Version service reloaded")
	return nil
}
