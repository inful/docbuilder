package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
	"strings"
	"strconv"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// DaemonStatus represents the current state of the daemon
type DaemonStatus string

const (
	StatusStopped  DaemonStatus = "stopped"
	StatusStarting DaemonStatus = "starting"
	StatusRunning  DaemonStatus = "running"
	StatusStopping DaemonStatus = "stopping"
	StatusError    DaemonStatus = "error"
)

// Daemon represents the main daemon service
type Daemon struct {
	config    *config.Config
	status    atomic.Value // DaemonStatus
	startTime time.Time
	stopChan  chan struct{}
	mu        sync.RWMutex

	// Core components
	forgeManager   *forge.ForgeManager
	discovery      *forge.DiscoveryService
	versionService *versioning.VersionService
	configWatcher  *ConfigWatcher
	metrics        *MetricsCollector
	httpServer     *HTTPServer
	scheduler      *Scheduler
	buildQueue     *BuildQueue
	stateManager   *StateManager

	// Runtime state
	activeJobs    int32
	queueLength   int32
	lastBuild     *time.Time
	lastDiscovery *time.Time

	// Cached discovery data to serve /status quickly without doing network I/O each request
	discoveryCacheMu    sync.RWMutex
	lastDiscoveryResult *forge.DiscoveryResult
	lastDiscoveryError  error
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
		config:   cfg,
		stopChan: make(chan struct{}),
		metrics:  NewMetricsCollector(),
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

	// Initialize build queue first (scheduler needs it)
	daemon.buildQueue = NewBuildQueue(cfg.Daemon.Sync.QueueSize, cfg.Daemon.Sync.ConcurrentBuilds)
	// Configure retry policy from build config (recorder injection handled elsewhere if added later)
	daemon.buildQueue.ConfigureRetry(cfg.Build)

	// Initialize scheduler (after build queue)
	daemon.scheduler = NewScheduler(daemon.buildQueue)

	// Initialize state manager
	var err error
	stateDir := cfg.Daemon.Storage.RepoCacheDir
	if stateDir == "" {
		stateDir = "./daemon-data" // Default data directory
	}
	daemon.stateManager, err = NewStateManager(stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
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

	// Save state
	if d.stateManager != nil {
		if err := d.stateManager.Save(); err != nil {
			slog.Error("Failed to save state", "error", err)
		}
	}

	d.status.Store(StatusStopped)

	uptime := time.Since(d.startTime)
	slog.Info("DocBuilder daemon stopped", slog.Duration("uptime", uptime))

	return nil
}

// GetStatus returns the current daemon status
func (d *Daemon) GetStatus() DaemonStatus {
	status, ok := d.status.Load().(DaemonStatus)
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

// TriggerDiscovery manually triggers repository discovery
func (d *Daemon) TriggerDiscovery() string {
	if d.GetStatus() != StatusRunning {
		return ""
	}

	jobID := fmt.Sprintf("discovery-%d", time.Now().Unix())

	go func() {
		atomic.AddInt32(&d.activeJobs, 1)
		defer atomic.AddInt32(&d.activeJobs, -1)

		slog.Info("Manual discovery triggered", logfields.JobID(jobID))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := d.runDiscovery(ctx); err != nil {
			slog.Error("Discovery failed", logfields.JobID(jobID), logfields.Error(err))
		} else {
			slog.Info("Discovery completed", logfields.JobID(jobID))
		}
	}()

	return jobID
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
			go d.safeRunDiscovery()
		case <-discoveryTicker.C:
			slog.Info("Scheduled discovery tick", slog.Duration("interval", discoveryInterval))
			go d.safeRunDiscovery()
		}
	}
}

// parseDiscoverySchedule parses a schedule expression into an approximate interval.
// Supported forms:
//   @every <duration>   (same semantics as Go duration parsing, e.g. @every 5m, @every 1h30m)
//   Standard 5-field cron patterns (minute hour day month weekday) for a few common forms:
//     */5 * * * *   -> 5m
//     */15 * * * *  -> 15m
//     0 * * * *     -> 1h (top of every hour)
//     0 0 * * *     -> 24h (midnight daily)
//     */30 * * * *  -> 30m
//   If expression not recognized returns (0,false).
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
		atomic.StoreInt32(&d.queueLength, int32(d.buildQueue.Length()))
	}

	// Periodic state save
	if d.stateManager != nil {
		if err := d.stateManager.Save(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}
	}
}

// runDiscovery executes repository discovery across all forges
func (d *Daemon) runDiscovery(ctx context.Context) error {
	start := time.Now()
	d.metrics.IncrementCounter("discovery_attempts")

	slog.Info("Starting repository discovery")

	result, err := d.discovery.DiscoverAll(ctx)
	if err != nil {
		d.metrics.IncrementCounter("discovery_errors")
		// Cache the error so status endpoint can report it fast
		d.discoveryCacheMu.Lock()
		d.lastDiscoveryError = err
		d.discoveryCacheMu.Unlock()
		return fmt.Errorf("discovery failed: %w", err)
	}

	duration := time.Since(start)
	d.metrics.RecordHistogram("discovery_duration_seconds", duration.Seconds())
	d.metrics.IncrementCounter("discovery_successes")
	d.metrics.SetGauge("repositories_discovered", int64(len(result.Repositories)))
	d.metrics.SetGauge("repositories_filtered", int64(len(result.Filtered)))
	now := time.Now()
	d.lastDiscovery = &now

	// Cache successful discovery result for status queries
	d.discoveryCacheMu.Lock()
	d.lastDiscoveryResult = result
	d.lastDiscoveryError = nil
	d.discoveryCacheMu.Unlock()

	slog.Info("Repository discovery completed",
		slog.Duration("duration", duration),
		slog.Int("repositories_found", len(result.Repositories)),
		slog.Int("repositories_filtered", len(result.Filtered)),
		slog.Int("errors", len(result.Errors)))

	// Store discovery results in state
	if d.stateManager != nil {
		// Record discovery for each repository
		for _, repo := range result.Repositories {
			// For now, record with 0 documents as we don't have that info from forge discovery
			// This would be updated later during actual document discovery
			d.stateManager.RecordDiscovery(repo.CloneURL, 0)
		}
	}

	// Trigger build if new repositories were found
	if len(result.Repositories) > 0 {
		// Convert discovered repositories to config.Repository for build usage
		converted := d.discovery.ConvertToConfigRepositories(result.Repositories, d.forgeManager)
		job := &BuildJob{
			ID:        fmt.Sprintf("auto-build-%d", time.Now().Unix()),
			Type:      BuildTypeDiscovery,
			Priority:  PriorityNormal,
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"discovery_result": result,
				"repositories":     converted,
				"v2_config":        d.config,
			},
		}

		if err := d.buildQueue.Enqueue(job); err != nil {
			slog.Error("Failed to enqueue auto-build", logfields.Error(err))
		}
	}

	return nil
}

// GetConfig returns the current daemon configuration
func (d *Daemon) GetConfig() *config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// safeRunDiscovery executes discovery with a timeout and panic protection
func (d *Daemon) safeRunDiscovery() {
	if d.discovery == nil {
		return
	}
	// Skip if daemon not running
	if d.GetStatus() != StatusRunning {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered from panic in safeRunDiscovery", "panic", r)
		}
	}()
	if err := d.runDiscovery(ctx); err != nil {
		slog.Warn("Periodic discovery failed", "error", err)
	} else {
		slog.Info("Periodic discovery completed")
	}
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
	return nil
}

// reloadForgeManager updates the forge manager with new forge configurations
func (d *Daemon) reloadForgeManager(ctx context.Context, oldConfig, newConfig *config.Config) error {
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

	slog.Info("Forge manager reloaded", slog.Int("forge_count", len(newConfig.Forges)))
	return nil
}

// reloadVersionService updates the version service with new versioning configuration
func (d *Daemon) reloadVersionService(ctx context.Context, oldConfig, newConfig *config.Config) error {
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
