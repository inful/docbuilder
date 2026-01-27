package discoveryrunner

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// Discovery is the minimal interface required to run forge discovery.
//
// The concrete implementation is typically *forge.DiscoveryService.
type Discovery interface {
	DiscoverAll(ctx context.Context) (*forge.DiscoveryResult, error)
	ConvertToConfigRepositories(repos []*forge.Repository, forgeManager *forge.Manager) []config.Repository
}

// Metrics is the minimal interface used by the runner to record discovery metrics.
type Metrics interface {
	IncrementCounter(name string)
	RecordHistogram(name string, value float64)
	SetGauge(name string, value int64)
}

// StateManager is the minimal interface used for persistence and discovery bookkeeping.
type StateManager interface {
	services.StateManager
	EnsureRepositoryState(url, name, branch string)
	RecordDiscovery(repoURL string, documentCount int)
}

// Enqueuer is the minimal interface required to enqueue build jobs.
type Enqueuer interface {
	Enqueue(job *queue.BuildJob) error
}

// BuildRequester is an optional hook used to request a build without directly
// enqueueing a build job. This supports higher-level orchestration (ADR-021).
//
// If set, the runner will call it instead of enqueuing a queue.BuildJob.
type BuildRequester func(ctx context.Context, jobID, reason string)

// RepoRemovedNotifier is an optional hook invoked when a repository that existed
// in the previous discovery result is missing from the latest one.
//
// This supports higher-level orchestration (ADR-021) without hard-coupling the
// discovery runner to the daemon package.
type RepoRemovedNotifier func(ctx context.Context, repoURL, repoName string)

// Config holds the dependencies for creating a Runner.
type Config struct {
	Discovery      Discovery
	ForgeManager   *forge.Manager
	DiscoveryCache *Cache
	Metrics        Metrics
	StateManager   StateManager
	BuildQueue     Enqueuer
	BuildRequester BuildRequester
	RepoRemoved    RepoRemovedNotifier
	LiveReload     queue.LiveReloadHub
	Config         *config.Config

	// Now allows tests to inject deterministic time.
	Now func() time.Time
	// NewJobID allows tests to inject deterministic job IDs.
	NewJobID func() string
}

// Runner encapsulates the logic for running repository discovery
// across all configured forges and triggering builds for discovered repositories.
type Runner struct {
	discovery      Discovery
	forgeManager   *forge.Manager
	discoveryCache *Cache
	metrics        Metrics
	stateManager   StateManager
	buildQueue     Enqueuer
	buildRequester BuildRequester
	repoRemoved    RepoRemovedNotifier
	liveReload     queue.LiveReloadHub
	config         *config.Config

	now      func() time.Time
	newJobID func() string

	lastDiscovery *time.Time
}

// New creates a new Runner.
func New(cfg Config) *Runner {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	newJobID := cfg.NewJobID
	if newJobID == nil {
		newJobID = func() string {
			return fmt.Sprintf("auto-build-%d", time.Now().Unix())
		}
	}

	return &Runner{
		discovery:      cfg.Discovery,
		forgeManager:   cfg.ForgeManager,
		discoveryCache: cfg.DiscoveryCache,
		metrics:        cfg.Metrics,
		stateManager:   cfg.StateManager,
		buildQueue:     cfg.BuildQueue,
		buildRequester: cfg.BuildRequester,
		repoRemoved:    cfg.RepoRemoved,
		liveReload:     cfg.LiveReload,
		config:         cfg.Config,
		now:            now,
		newJobID:       newJobID,
		lastDiscovery:  nil,
	}
}

// Run executes repository discovery across all forges.
// It updates the discovery cache with results/errors and triggers builds
// for newly discovered repositories.
func (r *Runner) Run(ctx context.Context) error {
	if r.discovery == nil {
		return nil
	}

	var prevDiscovered map[string]string
	if r.discoveryCache != nil {
		prev := r.discoveryCache.GetResult()
		if prev != nil && len(prev.Repositories) > 0 {
			prevDiscovered = make(map[string]string, len(prev.Repositories))
			for _, repo := range prev.Repositories {
				if repo == nil || repo.CloneURL == "" {
					continue
				}
				prevDiscovered[repo.CloneURL] = repo.Name
			}
		}
	}

	start := time.Now()
	if r.metrics != nil {
		r.metrics.IncrementCounter("discovery_attempts")
	}

	slog.Info("Starting repository discovery")

	result, err := r.discovery.DiscoverAll(ctx)
	if err != nil {
		if r.metrics != nil {
			r.metrics.IncrementCounter("discovery_errors")
		}
		if r.discoveryCache != nil {
			r.discoveryCache.SetError(err)
		}
		return fmt.Errorf("discovery failed: %w", err)
	}

	duration := time.Since(start)
	if r.metrics != nil {
		r.metrics.RecordHistogram("discovery_duration_seconds", duration.Seconds())
		r.metrics.IncrementCounter("discovery_successes")
		r.metrics.SetGauge("repositories_discovered", int64(len(result.Repositories)))
		r.metrics.SetGauge("repositories_filtered", int64(len(result.Filtered)))
	}

	now := r.now()
	r.lastDiscovery = &now

	if r.discoveryCache != nil {
		r.discoveryCache.Update(result)
	}

	if r.repoRemoved != nil && len(prevDiscovered) > 0 {
		current := make(map[string]struct{}, len(result.Repositories))
		for _, repo := range result.Repositories {
			if repo == nil || repo.CloneURL == "" {
				continue
			}
			current[repo.CloneURL] = struct{}{}
		}
		for url, name := range prevDiscovered {
			if _, ok := current[url]; ok {
				continue
			}
			r.repoRemoved(ctx, url, name)
		}
	}

	slog.Info("Repository discovery completed",
		slog.Duration("duration", duration),
		slog.Int("repositories_found", len(result.Repositories)),
		slog.Int("repositories_filtered", len(result.Filtered)),
		slog.Int("errors", len(result.Errors)))

	if r.stateManager != nil {
		for _, repo := range result.Repositories {
			// Record discovered repositories in state so the daemon can surface them
			// even before a build has produced per-repo doc metadata.
			if init, ok := r.stateManager.(interface {
				EnsureRepositoryState(url, name, branch string)
			}); ok {
				init.EnsureRepositoryState(repo.CloneURL, repo.Name, repo.DefaultBranch)
			}
			// For now, record with 0 documents as we don't have that info from forge discovery.
			r.stateManager.RecordDiscovery(repo.CloneURL, 0)
		}
	}

	if len(result.Repositories) > 0 && r.shouldBuildOnDiscovery() {
		r.triggerBuildForDiscoveredRepos(ctx, result)
	}

	return nil
}

func (r *Runner) shouldBuildOnDiscovery() bool {
	// Preserve historical behavior: discovery enqueues a build by default.
	if r.config == nil || r.config.Daemon == nil {
		return true
	}
	if r.config.Daemon.Sync.BuildOnDiscovery == nil {
		return true
	}
	return *r.config.Daemon.Sync.BuildOnDiscovery
}

func (r *Runner) triggerBuildForDiscoveredRepos(ctx context.Context, result *forge.DiscoveryResult) {
	if ctx == nil {
		return
	}

	jobID := r.newJobID()
	if r.buildRequester != nil {
		r.buildRequester(ctx, jobID, "discovery")
		return
	}

	if r.buildQueue == nil {
		return
	}

	converted := r.discovery.ConvertToConfigRepositories(result.Repositories, r.forgeManager)
	job := &queue.BuildJob{
		ID:        jobID,
		Type:      queue.BuildTypeDiscovery,
		Priority:  queue.PriorityNormal,
		CreatedAt: r.now(),
		TypedMeta: &queue.BuildJobMetadata{
			V2Config:      r.config,
			Repositories:  converted,
			StateManager:  r.stateManager,
			LiveReloadHub: r.liveReload,
		},
	}

	if err := r.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue auto-build", logfields.Error(err))
	}
}

// SafeRun executes discovery with a timeout and panic protection.
// It is suitable for use in goroutines.
func (r *Runner) SafeRun(ctx context.Context, shouldRun func() bool) {
	if r.discovery == nil {
		return
	}
	if shouldRun != nil && !shouldRun() {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("Recovered from panic in SafeRun", "panic", rec)
		}
	}()

	if err := r.Run(timeoutCtx); err != nil {
		slog.Warn("Periodic discovery failed", "error", err)
	} else {
		slog.Info("Periodic discovery completed")
	}
}

// TriggerManual triggers a manual discovery run in a separate goroutine.
// Returns the job ID for tracking.
func (r *Runner) TriggerManual(shouldRun func() bool, activeJobs *int32) string {
	if shouldRun != nil && !shouldRun() {
		return ""
	}

	jobID := fmt.Sprintf("discovery-%d", time.Now().Unix())

	go func() {
		if activeJobs != nil {
			atomic.AddInt32(activeJobs, 1)
			defer atomic.AddInt32(activeJobs, -1)
		}

		slog.Info("Manual discovery triggered", logfields.JobID(jobID))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := r.Run(ctx); err != nil {
			slog.Error("Discovery failed", logfields.JobID(jobID), logfields.Error(err))
		} else {
			slog.Info("Discovery completed", logfields.JobID(jobID))
		}
	}()

	return jobID
}

// GetLastDiscovery returns the time of the last successful discovery.
func (r *Runner) GetLastDiscovery() *time.Time {
	return r.lastDiscovery
}

// UpdateConfig updates the configuration used for discovery.
func (r *Runner) UpdateConfig(cfg *config.Config) {
	r.config = cfg
}

// UpdateDiscoveryService updates the discovery service (used during config reload).
func (r *Runner) UpdateDiscoveryService(discovery Discovery) {
	r.discovery = discovery
}

// UpdateForgeManager updates the forge manager (used during config reload).
func (r *Runner) UpdateForgeManager(forgeManager *forge.Manager) {
	r.forgeManager = forgeManager
}
