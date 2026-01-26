package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// mainLoop runs the main daemon processing loop.
func (d *Daemon) mainLoop(ctx context.Context) {
	initialDiscoveryTimer := time.NewTimer(3 * time.Second)
	defer initialDiscoveryTimer.Stop()

	// If explicit repositories are configured (no forges), trigger an immediate build
	if len(d.config.Repositories) > 0 && len(d.config.Forges) == 0 {
		slog.Info("Explicit repositories configured, triggering initial build", slog.Int("repositories", len(d.config.Repositories)))
		go func() {
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
		case <-initialDiscoveryTimer.C:
			go d.discoveryRunner.SafeRun(ctx, func() bool { return d.GetStatus() == StatusRunning })
		}
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
