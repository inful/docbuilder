package daemon

import (
	"context"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (d *Daemon) onDiscoveryBuildRequest(ctx context.Context, jobID, reason string) {
	if d == nil || ctx == nil {
		return
	}
	if d.orchestrationBus == nil {
		return
	}
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}

	pubErr := d.publishOrchestrationEvent(ctx, events.BuildRequested{
		JobID:       jobID,
		Reason:      reason,
		RequestedAt: time.Now(),
	})
	if pubErr != nil {
		slog.Warn("Failed to publish discovery build request",
			logfields.JobID(jobID),
			slog.String("reason", reason),
			logfields.Error(pubErr))
	}
}

func (d *Daemon) onDiscoveryRepoRemoved(ctx context.Context, repoURL, repoName string) {
	if d == nil || ctx == nil {
		return
	}
	if d.orchestrationBus == nil {
		return
	}

	pubErr := d.publishOrchestrationEvent(ctx, events.RepoRemoved{
		RepoURL:    repoURL,
		RepoName:   repoName,
		RemovedAt:  time.Now(),
		Discovered: true,
	})
	if pubErr != nil {
		slog.Warn("Failed to publish repo removed event",
			slog.String("repo", repoName),
			slog.String("repo_url", repoURL),
			logfields.Error(pubErr))
	}
}
