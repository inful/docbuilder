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
	// The preview daemon has no discovery: the trigger would dereference
	// a nil runner and kill the process.
	if d.discover.discoveryRunner == nil {
		return ""
	}
	return d.discover.discoveryRunner.TriggerManual(func() bool { return d.GetStatus() == StatusRunning })
}

// publishTimeout bounds the wait for the subscribers of the bus. The
// request that asked for the build waits for it, and a subscriber that
// does not take the event must not hold the request for ever.
const publishTimeout = 30 * time.Second

// TriggerBuild manually triggers a site build.
func (d *Daemon) TriggerBuild() string {
	if d.GetStatus() != StatusRunning {
		return ""
	}
	if d.serve.orchestrationBus == nil {
		return ""
	}

	jobID := ""
	if d.build.buildDebouncer != nil {
		if planned, ok := d.build.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("manual-%d", time.Now().UnixNano())
	}

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	if err := d.publishOrchestrationEvent(ctx, events.BuildRequested{
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
	if d.serve.orchestrationBus == nil {
		return ""
	}
	jobID := ""
	if d.build.buildDebouncer != nil {
		if planned, ok := d.build.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("webhook-%d", time.Now().UnixNano())
	}

	filesCopy := append([]string(nil), changedFiles...)

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	if err := d.publishOrchestrationEvent(ctx, events.WebhookReceived{
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
	if d.serve.orchestrationBus == nil {
		return
	}

	jobID := ""
	if d.build.buildDebouncer != nil {
		if planned, ok := d.build.buildDebouncer.PlannedJobID(); ok {
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
