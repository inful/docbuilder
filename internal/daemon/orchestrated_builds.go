package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (d *Daemon) runBuildNowConsumer(ctx context.Context) {
	if ctx == nil || d == nil || d.orchestrationBus == nil {
		return
	}

	buildNowCh, unsubscribe := events.Subscribe[events.BuildNow](d.orchestrationBus, 16)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-buildNowCh:
			if !ok {
				return
			}
			d.enqueueOrchestratedBuild(evt)
		}
	}
}

func (d *Daemon) enqueueOrchestratedBuild(evt events.BuildNow) {
	if d == nil || d.GetStatus() != StatusRunning || d.buildQueue == nil {
		return
	}

	reposForBuild := d.currentReposForOrchestratedBuild()
	if len(reposForBuild) == 0 {
		slog.Warn("Skipping orchestrated build: no repositories available")
		return
	}

	jobID := evt.JobID
	if jobID == "" {
		jobID = fmt.Sprintf("orchestrated-build-%d", time.Now().UnixNano())
	}

	meta := &BuildJobMetadata{
		V2Config:      d.config,
		Repositories:  reposForBuild,
		StateManager:  d.stateManager,
		LiveReloadHub: d.liveReload,
	}
	if evt.LastRepoURL != "" && evt.LastReason != "" {
		meta.DeltaRepoReasons = map[string]string{
			evt.LastRepoURL: fmt.Sprintf("%s (%s)", evt.LastReason, evt.DebounceCause),
		}
	}

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeManual,
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: meta,
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue orchestrated build",
			logfields.JobID(jobID),
			logfields.Error(err))
		return
	}

	atomic.AddInt32(&d.queueLength, 1)
	slog.Info("Orchestrated build enqueued",
		logfields.JobID(jobID),
		slog.Int("repositories", len(reposForBuild)))
}

func (d *Daemon) currentReposForOrchestratedBuild() []config.Repository {
	if d == nil || d.config == nil {
		return nil
	}

	// Explicit repo mode.
	if len(d.config.Repositories) > 0 {
		return append([]config.Repository{}, d.config.Repositories...)
	}

	// Forge mode: prefer the last discovery result.
	discovered, err := d.GetDiscoveryResult()
	if err == nil && discovered != nil && d.discovery != nil {
		return d.discovery.ConvertToConfigRepositories(discovered.Repositories, d.forgeManager)
	}

	return nil
}
