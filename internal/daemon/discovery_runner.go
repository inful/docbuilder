package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// DiscoveryRunner encapsulates the logic for running repository discovery
// across all configured forges and triggering builds for discovered repositories.
type DiscoveryRunner struct {
	discovery      *forge.DiscoveryService
	forgeManager   *forge.Manager
	discoveryCache *DiscoveryCache
	metrics        *MetricsCollector
	stateManager   state.DaemonStateManager
	buildQueue     *BuildQueue
	liveReload     *LiveReloadHub
	config         *config.Config

	// Tracking
	lastDiscovery *time.Time
}

// DiscoveryRunnerConfig holds the dependencies for creating a DiscoveryRunner.
type DiscoveryRunnerConfig struct {
	Discovery      *forge.DiscoveryService
	ForgeManager   *forge.Manager
	DiscoveryCache *DiscoveryCache
	Metrics        *MetricsCollector
	StateManager   state.DaemonStateManager
	BuildQueue     *BuildQueue
	LiveReload     *LiveReloadHub
	Config         *config.Config
}

// NewDiscoveryRunner creates a new DiscoveryRunner.
func NewDiscoveryRunner(cfg DiscoveryRunnerConfig) *DiscoveryRunner {
	return &DiscoveryRunner{
		discovery:      cfg.Discovery,
		forgeManager:   cfg.ForgeManager,
		discoveryCache: cfg.DiscoveryCache,
		metrics:        cfg.Metrics,
		stateManager:   cfg.StateManager,
		buildQueue:     cfg.BuildQueue,
		liveReload:     cfg.LiveReload,
		config:         cfg.Config,
	}
}

// Run executes repository discovery across all forges.
// It updates the discovery cache with results/errors and triggers builds
// for newly discovered repositories.
func (dr *DiscoveryRunner) Run(ctx context.Context) error {
	start := time.Now()
	dr.metrics.IncrementCounter("discovery_attempts")

	slog.Info("Starting repository discovery")

	result, err := dr.discovery.DiscoverAll(ctx)
	if err != nil {
		dr.metrics.IncrementCounter("discovery_errors")
		// Cache the error so status endpoint can report it fast
		dr.discoveryCache.SetError(err)
		return fmt.Errorf("discovery failed: %w", err)
	}

	duration := time.Since(start)
	dr.metrics.RecordHistogram("discovery_duration_seconds", duration.Seconds())
	dr.metrics.IncrementCounter("discovery_successes")
	dr.metrics.SetGauge("repositories_discovered", int64(len(result.Repositories)))
	dr.metrics.SetGauge("repositories_filtered", int64(len(result.Filtered)))
	now := time.Now()
	dr.lastDiscovery = &now

	// Cache successful discovery result for status queries
	dr.discoveryCache.Update(result)

	slog.Info("Repository discovery completed",
		slog.Duration("duration", duration),
		slog.Int("repositories_found", len(result.Repositories)),
		slog.Int("repositories_filtered", len(result.Filtered)),
		slog.Int("errors", len(result.Errors)))

	// Store discovery results in state
	if dr.stateManager != nil {
		// Record discovery for each repository
		for _, repo := range result.Repositories {
			// For now, record with 0 documents as we don't have that info from forge discovery
			// This would be updated later during actual document discovery
			dr.stateManager.RecordDiscovery(repo.CloneURL, 0)
		}
	}

	// Trigger build if new repositories were found
	if len(result.Repositories) > 0 {
		dr.triggerBuildForDiscoveredRepos(result)
	}

	return nil
}

// triggerBuildForDiscoveredRepos enqueues a build job for discovered repositories.
func (dr *DiscoveryRunner) triggerBuildForDiscoveredRepos(result *forge.DiscoveryResult) {
	// Convert discovered repositories to config.Repository for build usage
	converted := dr.discovery.ConvertToConfigRepositories(result.Repositories, dr.forgeManager)
	job := &BuildJob{
		ID:        fmt.Sprintf("auto-build-%d", time.Now().Unix()),
		Type:      BuildTypeDiscovery,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      dr.config,
			Repositories:  converted,
			StateManager:  dr.stateManager,
			LiveReloadHub: dr.liveReload,
		},
	}

	if err := dr.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue auto-build", logfields.Error(err))
	}
}

// SafeRun executes discovery with a timeout and panic protection.
// It is suitable for use in goroutines.
func (dr *DiscoveryRunner) SafeRun(daemonStatus func() Status) {
	if dr.discovery == nil {
		return
	}
	// Skip if daemon not running
	if daemonStatus() != StatusRunning {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered from panic in SafeRun", "panic", r)
		}
	}()
	if err := dr.Run(ctx); err != nil {
		slog.Warn("Periodic discovery failed", "error", err)
	} else {
		slog.Info("Periodic discovery completed")
	}
}

// TriggerManual triggers a manual discovery run in a separate goroutine.
// Returns the job ID for tracking.
func (dr *DiscoveryRunner) TriggerManual(daemonStatus func() Status, activeJobs *int32) string {
	if daemonStatus() != StatusRunning {
		return ""
	}

	jobID := fmt.Sprintf("discovery-%d", time.Now().Unix())

	go func() {
		atomic.AddInt32(activeJobs, 1)
		defer atomic.AddInt32(activeJobs, -1)

		slog.Info("Manual discovery triggered", logfields.JobID(jobID))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := dr.Run(ctx); err != nil {
			slog.Error("Discovery failed", logfields.JobID(jobID), logfields.Error(err))
		} else {
			slog.Info("Discovery completed", logfields.JobID(jobID))
		}
	}()

	return jobID
}

// GetLastDiscovery returns the time of the last successful discovery.
func (dr *DiscoveryRunner) GetLastDiscovery() *time.Time {
	return dr.lastDiscovery
}

// UpdateConfig updates the configuration used for discovery.
func (dr *DiscoveryRunner) UpdateConfig(cfg *config.Config) {
	dr.config = cfg
}

// UpdateDiscoveryService updates the discovery service (used during config reload).
func (dr *DiscoveryRunner) UpdateDiscoveryService(discovery *forge.DiscoveryService) {
	dr.discovery = discovery
}

// UpdateForgeManager updates the forge manager (used during config reload).
func (dr *DiscoveryRunner) UpdateForgeManager(forgeManager *forge.Manager) {
	dr.forgeManager = forgeManager
}
