package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// TriggerDiscovery manually triggers repository discovery.
func (d *Daemon) TriggerDiscovery() string {
	return d.discoveryRunner.TriggerManual(func() bool { return d.GetStatus() == StatusRunning }, &d.activeJobs)
}

// TriggerBuild manually triggers a site build.
func (d *Daemon) TriggerBuild() string {
	if d.GetStatus() != StatusRunning {
		return ""
	}
	if d.orchestrationBus == nil {
		return ""
	}

	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("manual-%d", time.Now().UnixNano())
	}

	if err := d.publishOrchestrationEvent(context.Background(), events.BuildRequested{
		JobID:       jobID,
		Immediate:   true,
		Reason:      "manual",
		RequestedAt: time.Now(),
	}); err != nil {
		slog.Warn("Failed to publish manual build request",
			logfields.JobID(jobID),
			logfields.Error(err))
		return ""
	}

	slog.Info("Manual build requested", logfields.JobID(jobID))
	return jobID
}

// TriggerWebhookBuild processes a webhook event and requests an orchestrated build.
//
// The webhook payload is used to decide whether a build should be requested and which
// repository should be treated as "changed", but it does not narrow the site scope:
// the build remains a canonical full-site build.
//
// forgeName is optional; callers may pass an empty string when the webhook is not
// scoped to a specific configured forge instance.

func (d *Daemon) TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string {
	if d.GetStatus() != StatusRunning {
		return ""
	}
	if d.orchestrationBus == nil {
		return ""
	}
	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("webhook-%d", time.Now().UnixNano())
	}

	filesCopy := append([]string(nil), changedFiles...)

	if err := d.publishOrchestrationEvent(context.Background(), events.WebhookReceived{
		JobID:        jobID,
		ForgeName:    forgeName,
		RepoFullName: repoFullName,
		Branch:       branch,
		ChangedFiles: filesCopy,
		ReceivedAt:   time.Now(),
	}); err != nil {
		slog.Warn("Failed to publish webhook received event",
			logfields.JobID(jobID),
			logfields.Error(err),
			slog.String("forge", forgeName),
			slog.String("repo", repoFullName),
			slog.String("branch", branch))
		return ""
	}

	slog.Info("Webhook received",
		logfields.JobID(jobID),
		slog.String("forge", forgeName),
		slog.String("repo", repoFullName),
		slog.String("branch", branch),
		slog.Int("changed_files", len(filesCopy)))
	return jobID
}

// triggerScheduledBuildForExplicitRepos triggers a scheduled build for explicitly configured repositories.
func (d *Daemon) triggerScheduledBuildForExplicitRepos(ctx context.Context) {
	if d.GetStatus() != StatusRunning {
		return
	}
	if ctx == nil {
		return
	}
	if d.orchestrationBus == nil {
		return
	}

	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("scheduled-build-%d", time.Now().Unix())
	}
	if err := d.publishOrchestrationEvent(ctx, events.BuildRequested{
		JobID:       jobID,
		Reason:      "scheduled build",
		RequestedAt: time.Now(),
	}); err != nil {
		slog.Warn("Failed to publish scheduled build request",
			logfields.JobID(jobID),
			logfields.Error(err))
		return
	}
	slog.Info("Scheduled build requested",
		logfields.JobID(jobID),
		slog.Int("repositories", len(d.config.Repositories)))
}
