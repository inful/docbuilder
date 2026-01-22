package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// mainLoop runs the main daemon processing loop.
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
			go d.discoveryRunner.SafeRun(ctx, d.GetStatus)
		case <-discoveryTicker.C:
			slog.Info("Scheduled tick", slog.Duration("interval", discoveryInterval))
			// For forge-based discovery, run discovery
			if len(d.config.Forges) > 0 {
				go d.discoveryRunner.SafeRun(ctx, d.GetStatus)
			}
			// For explicit repositories, trigger a build to check for updates
			if len(d.config.Repositories) > 0 {
				go d.triggerScheduledBuildForExplicitRepos()
			}
		}
	}
}

// parseDiscoverySchedule parses a schedule expression into an approximate interval.
// Supported forms:
//
//   - @every <duration>   (same semantics as Go duration parsing, e.g. @every 5m, @every 1h30m)
//   - Standard 5-field cron patterns (minute hour day month weekday) for a few common forms:
//     */5 * * * *   -> 5m
//     */15 * * * *  -> 15m
//     0 * * * *     -> 1h (top of every hour)
//     0 0 * * *     -> 24h (midnight daily)
//     */30 * * * *  -> 30m
//
// If expression not recognized returns (0,false).
func parseDiscoverySchedule(expr string) (time.Duration, bool) {
	// @every form
	if after, ok := strings.CutPrefix(expr, "@every "); ok {
		rem := strings.TrimSpace(after)
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
		if after, ok := strings.CutPrefix(parts[0], "*/"); ok {
			val := after
			if n, err := strconv.Atoi(val); err == nil && n > 0 && n < 60 {
				return time.Duration(n) * time.Minute, true
			}
		}
	}
	return 0, false
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
