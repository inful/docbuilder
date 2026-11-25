package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// runBuildNowConsumer subscribes to the build-now events of the bus and
// consumes them.
//
// The daemon subscribes before it starts the goroutine that consumes
// them (see startWorkers): the bus has no queue, and an event that is
// published before the consumer is subscribed is lost.
func (d *Daemon) runBuildNowConsumer(ctx context.Context) {
	if ctx == nil || d == nil || d.serve.orchestrationBus == nil {
		return
	}

	buildNowCh, unsubscribe := events.Subscribe[events.BuildNow](d.serve.orchestrationBus, 16)
	d.consumeBuildNow(ctx, buildNowCh, unsubscribe)
}

// consumeBuildNow turns the build-now events of the bus into builds.
func (d *Daemon) consumeBuildNow(ctx context.Context, buildNowCh <-chan events.BuildNow, unsubscribe func()) {
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-buildNowCh:
			if !ok {
				return
			}
			d.enqueueOrchestratedBuild(ctx, evt)
		}
	}
}

// buildQueueWaitTimeout bounds the wait for room in the build queue. A
// build that does not fit in that time is lost: the queue is full and
// the daemon is overloaded.
const buildQueueWaitTimeout = 30 * time.Second

func (d *Daemon) enqueueOrchestratedBuild(ctx context.Context, evt events.BuildNow) {
	if d == nil || d.GetStatus() != StatusRunning || d.build.buildQueue == nil {
		return
	}

	reposForBuild := d.currentReposForOrchestratedBuild()
	if len(reposForBuild) == 0 {
		if !d.build.afterRemoval.Swap(false) {
			slog.Warn("Skipping orchestrated build: no repositories available")
			return
		}
		// A repository was removed and no repository is left. The
		// build runs with an empty set: the pages of the removed
		// repository are still published, and only a build takes them
		// away.
		slog.Info("Building the site without a repository: a repository was removed")
	}

	if evt.LastRepoURL != "" && evt.LastBranch != "" {
		for i := range reposForBuild {
			repo := &reposForBuild[i]
			if repo.URL == evt.LastRepoURL {
				repo.Branch = evt.LastBranch
			}
		}
	}

	jobID := evt.JobID
	if jobID == "" {
		jobID = fmt.Sprintf("orchestrated-build-%d", time.Now().UnixNano())
	}

	meta := &queue.BuildJobMetadata{
		V2Config:      d.config,
		Repositories:  reposForBuild,
		RepoSnapshot:  evt.Snapshot,
		LiveReloadHub: d.optional.liveReload,
	}
	jobType := queue.BuildTypeManual
	switch evt.LastReason {
	case "webhook":
		jobType = queue.BuildTypeWebhook
	case "discovery":
		jobType = queue.BuildTypeDiscovery
	case "scheduled build":
		jobType = queue.BuildTypeScheduled
	}

	job := &queue.BuildJob{
		ID:        jobID,
		Type:      jobType,
		Priority:  queue.PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: meta,
	}

	// The queue may be full: the build waits for room instead of being
	// dropped. A site that is not built stays as it was.
	waitCtx, cancel := context.WithTimeout(ctx, buildQueueWaitTimeout)
	defer cancel()
	if err := d.build.buildQueue.EnqueueWait(waitCtx, job); err != nil {
		slog.Error("Failed to enqueue orchestrated build",
			logfields.JobID(jobID),
			logfields.Error(err))
		return
	}

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
	discovered, _ := d.GetDiscoveryResult()
	if discovered != nil && d.discover.discovery != nil {
		return d.discover.discovery.ConvertToConfigRepositories(discovered.Repositories, d.discover.forgeManager)
	}

	return nil
}
