package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
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
		go d.requestInitialBuild(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Main loop stopped by context cancellation")
			return
		case <-d.stopChan:
			slog.Info("Main loop stopped by stop signal")
			return
		case <-initialDiscoveryTimer.C:
			workCtx, cancel := d.workContext(ctx)
			go func() {
				defer cancel()
				d.discoveryRunner.SafeRun(workCtx, func() bool { return d.GetStatus() == StatusRunning })
			}()
		}
	}
}

func (d *Daemon) requestInitialBuild(ctx context.Context) {
	if ctx == nil {
		return
	}
	if d.orchestrationBus == nil {
		slog.Warn("Skipping initial build: orchestration bus not initialized")
		return
	}

	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("initial-build-%d", time.Now().UnixNano())
	}

	err := d.orchestrationBus.Publish(ctx, events.BuildRequested{
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
	}

	// Periodic state save
	if d.stateManager != nil {
		if err := d.stateManager.Save(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}
	}
}

// GetConfig returns the current daemon configuration.
func (d *Daemon) GetConfig() *config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}
