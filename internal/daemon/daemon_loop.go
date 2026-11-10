package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// mainLoop runs the main daemon processing loop.
func (d *Daemon) mainLoop(ctx context.Context) {
	initialDiscoveryTimer := time.NewTimer(3 * time.Second)
	defer initialDiscoveryTimer.Stop()

	// If explicit repositories are configured (no forges), trigger an immediate build
	if len(d.config.Repositories) > 0 && len(d.config.Forges) == 0 {
		slog.Info("Explicit repositories configured, triggering initial build", slog.Int("repositories", len(d.config.Repositories)))
		d.goWorker("initial_build", func() { d.requestInitialBuild(ctx) })
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Main loop stopped by context cancellation")
			return
		case <-d.lifecycle.stopChan:
			slog.Info("Main loop stopped by stop signal")
			return
		case <-initialDiscoveryTimer.C:
			workCtx, cancel := d.stopAwareContext(ctx)
			d.goWorker("initial_discovery", func() {
				defer cancel()
				runDiscoveryWithRetry(workCtx, d.initialDiscoveryAttempt, discoveryRetryBackoff)
			})
		}
	}
}

// discoveryRetryBackoff is the delay ladder used when the initial discovery
// fails. The daemon retries until discovery succeeds instead of waiting for
// the next cron tick (default: every 4 hours), so a transient forge outage
// does not leave the daemon without a build.
var discoveryRetryBackoff = []time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	5 * time.Minute,
}

// initialDiscoveryAttempt runs one discovery pass and reports whether it
// succeeded.
//
// A forge-level failure (unreachable forge, bad credentials) is reported
// inside the discovery result (result.Errors) rather than as an error
// return, so the cached result is inspected here. Without that check an
// unreachable forge looks like a successful discovery that found zero
// repositories: no build is triggered and the daemon sits idle until the
// next cron tick.
func (d *Daemon) initialDiscoveryAttempt(ctx context.Context) error {
	if err := d.discover.discoveryRunner.SafeRun(ctx, func() bool { return d.GetStatus() == StatusRunning }); err != nil {
		return err
	}
	if d.discover.discoveryCache == nil {
		return nil
	}
	result, cacheErr := d.discover.discoveryCache.Get()
	if cacheErr != nil {
		return cacheErr
	}
	if result == nil || len(result.Errors) == 0 {
		return nil
	}

	forges := make([]string, 0, len(result.Errors))
	for name := range result.Errors {
		forges = append(forges, name)
	}
	sort.Strings(forges)
	return fmt.Errorf("discovery failed for %d forge(s): %s", len(forges), strings.Join(forges, ", "))
}

// runDiscoveryWithRetry runs attempt until it succeeds, retrying after each
// failure with the given backoff ladder (the last entry repeats). It returns
// when the attempt succeeds or ctx is canceled.
func runDiscoveryWithRetry(ctx context.Context, attempt func(context.Context) error, backoff []time.Duration) {
	if len(backoff) == 0 {
		backoff = discoveryRetryBackoff
	}
	for i := 0; ; i++ {
		err := attempt(ctx)
		if err == nil {
			return
		}

		delay := backoff[len(backoff)-1]
		if i < len(backoff) {
			delay = backoff[i]
		}
		slog.Warn("Discovery failed; retrying",
			logfields.Error(err),
			slog.Duration("retry_in", delay),
			slog.Int("attempt", i+1))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (d *Daemon) requestInitialBuild(ctx context.Context) {
	if ctx == nil {
		return
	}
	if d.serve.orchestrationBus == nil {
		slog.Warn("Skipping initial build: orchestration bus not initialized")
		return
	}

	jobID := ""
	if d.build.buildDebouncer != nil {
		if planned, ok := d.build.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("initial-build-%d", time.Now().UnixNano())
	}

	err := d.publishOrchestrationEvent(ctx, events.BuildRequested{
		JobID:       jobID,
		Immediate:   true,
		Reason:      "initial build",
		RequestedAt: time.Now(),
	})
	if err != nil {
		slog.Error("Failed to request initial build", logfields.Error(err), logfields.JobID(jobID))
	}
}

// updateStatus updates runtime status and metrics.
func (d *Daemon) updateStatus() {
	d.lifecycle.mu.Lock()
	defer d.lifecycle.mu.Unlock()

	// Update queue length from build queue
	if d.build.buildQueue != nil {
		// Clamp to int32 range to avoid overflow warnings from linters and ensure atomic store safety
		n := d.build.buildQueue.Length()
		if n > math.MaxInt32 {
			n = math.MaxInt32
		} else if n < math.MinInt32 {
			n = math.MinInt32
		}
		// #nosec G115 -- value is clamped to int32 range above
		d.queueLength.Store(int32(n))
	}

	// Periodic state save
	if d.state.stateManager != nil {
		if err := d.state.stateManager.Save(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}
	}
}

// GetConfig returns the current daemon configuration.
func (d *Daemon) GetConfig() *config.Config {
	d.lifecycle.mu.RLock()
	defer d.lifecycle.mu.RUnlock()
	return d.config
}
