package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
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
	config    *config.V2Config
	status    atomic.Value // DaemonStatus
	startTime time.Time
	stopChan  chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex

	// Core components
	forgeManager *forge.ForgeManager
	discovery    *forge.DiscoveryService
	httpServer   *HTTPServer
	scheduler    *Scheduler
	buildQueue   *BuildQueue
	stateManager *StateManager

	// Runtime state
	activeJobs    int32
	queueLength   int32
	lastBuild     *time.Time
	lastDiscovery *time.Time
}

// NewDaemon creates a new daemon instance
func NewDaemon(cfg *config.V2Config) (*Daemon, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	if cfg.Daemon == nil {
		return nil, fmt.Errorf("daemon configuration is required")
	}

	daemon := &Daemon{
		config:   cfg,
		stopChan: make(chan struct{}),
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

	return daemon, nil
}

// Start starts the daemon and all its components
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.GetStatus() != StatusStopped {
		return fmt.Errorf("daemon is not in stopped state: %s", d.GetStatus())
	}

	d.status.Store(StatusStarting)
	d.startTime = time.Now()

	slog.Info("Starting DocBuilder daemon", "version", "2.0")

	// Load persistent state
	if err := d.stateManager.Load(); err != nil {
		slog.Warn("Failed to load state", "error", err)
	}

	// Start HTTP servers
	if err := d.httpServer.Start(ctx); err != nil {
		d.status.Store(StatusError)
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start build queue processing
	d.buildQueue.Start(ctx)

	// Start scheduler
	d.scheduler.Start(ctx)

	d.status.Store(StatusRunning)

	slog.Info("DocBuilder daemon started successfully",
		"uptime", 0,
		"forges", len(d.config.Forges),
		"docs_port", d.config.Daemon.HTTP.DocsPort,
		"admin_port", d.config.Daemon.HTTP.AdminPort,
		"webhook_port", d.config.Daemon.HTTP.WebhookPort)

	// Run main daemon loop (blocks until stopped)
	d.mainLoop(ctx)

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

	// Signal stop to all components
	close(d.stopChan)

	// Stop components in reverse order
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
	slog.Info("DocBuilder daemon stopped", "uptime", uptime)

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

		slog.Info("Manual discovery triggered", "job_id", jobID)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := d.runDiscovery(ctx); err != nil {
			slog.Error("Discovery failed", "job_id", jobID, "error", err)
		} else {
			slog.Info("Discovery completed", "job_id", jobID)
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
		slog.Error("Failed to enqueue build job", "job_id", jobID, "error", err)
		return ""
	}

	atomic.AddInt32(&d.queueLength, 1)
	return jobID
}

// mainLoop runs the main daemon processing loop
func (d *Daemon) mainLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Status update interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Main loop stopped by context")
			return
		case <-d.stopChan:
			slog.Info("Main loop stopped by stop signal")
			return
		case <-ticker.C:
			d.updateStatus()
		}
	}
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

	slog.Info("Starting repository discovery")

	result, err := d.discovery.DiscoverAll(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	duration := time.Since(start)
	now := time.Now()
	d.lastDiscovery = &now

	slog.Info("Repository discovery completed",
		"duration", duration,
		"repositories_found", len(result.Repositories),
		"repositories_filtered", len(result.Filtered),
		"errors", len(result.Errors))

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
		job := &BuildJob{
			ID:        fmt.Sprintf("auto-build-%d", time.Now().Unix()),
			Type:      BuildTypeDiscovery,
			Priority:  PriorityNormal,
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"discovery_result": result,
			},
		}

		if err := d.buildQueue.Enqueue(job); err != nil {
			slog.Error("Failed to enqueue auto-build", "error", err)
		}
	}

	return nil
}
