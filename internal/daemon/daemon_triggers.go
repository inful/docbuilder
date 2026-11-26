package daemon

import (
	"context"
	"errors"
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
	return d.discover.discoveryRunner.TriggerManual(d.runContext(), func() bool { return d.GetStatus() == StatusRunning })
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
	if d.orchestrationBus() == nil {
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

// TriggerReingest asks for the whole corpus to be sent to ragabast
// again and returns the id of the build job that carries it.
//
// The epoch of the ingest moves: every document then carries a
// fingerprint that ragabast has not seen, so ragabast embeds the whole
// corpus again. That is what an operator needs when the embedder of
// ragabast changed, or when ragabast lost its corpus.
//
// The build runs even when nothing moved: the corpus is what is sent,
// not the site.
//
// The answer is empty when the daemon is not running, or when ragabast
// ingest is not configured.
func (d *Daemon) TriggerReingest() string {
	if d.GetStatus() != StatusRunning {
		return ""
	}
	dispatcher := d.optional.outboundDispatcher
	if dispatcher == nil {
		slog.Warn("Reingestion asked for, but the ragabast ingest is not configured")
		return ""
	}

	epoch, err := dispatcher.Reingest()
	if err != nil {
		slog.Error("Failed to move the epoch of the ingest", logfields.Error(err))
		return ""
	}

	// The next build must not be skipped: nothing moved, but the corpus
	// is what is sent. The flag stays until a build takes it, so a
	// build that could not be queued does not lose the reingestion.
	d.build.forceFullBuild.Store(true)

	jobID := d.TriggerBuild()
	if jobID == "" {
		slog.Warn("The reingestion moved the epoch of the ingest, but no build was queued",
			slog.Int64("ingest_epoch", epoch))
		return ""
	}
	slog.Info("Reingestion requested",
		slog.Int64("ingest_epoch", epoch),
		logfields.JobID(jobID))
	return jobID
}

// webhookRetryTimeout bounds the wait of a webhook whose consumer is
// busy. The event waits for room instead of being dropped: the forge
// does not send it again.
const webhookRetryTimeout = 30 * time.Second

// retryWebhookInBackground publishes a webhook that did not fit in the
// queue of the consumer. The daemon waits for room in the background,
// and the forge is told that the webhook was received.
func (d *Daemon) retryWebhookInBackground(evt events.WebhookReceived) {
	d.goWorker("webhook_retry", func() {
		ctx, cancel := context.WithTimeout(context.Background(), webhookRetryTimeout)
		defer cancel()

		if err := d.publishOrchestrationEventWithTimeout(ctx, evt, webhookRetryTimeout); err != nil {
			slog.Error("Failed to publish the webhook event",
				logfields.JobID(evt.JobID),
				logfields.Error(err),
				slog.String("forge", evt.ForgeName),
				slog.String("repo", evt.RepoFullName))
		}
	})
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
	if d.orchestrationBus() == nil {
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

	evt := events.WebhookReceived{
		JobID:        jobID,
		ForgeName:    forgeName,
		RepoFullName: repoFullName,
		Branch:       branch,
		ChangedFiles: filesCopy,
		ReceivedAt:   time.Now(),
	}
	if err := d.publishOrchestrationEvent(ctx, evt); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// The queue of the consumer is full: the webhook waits in
			// the background for room. The forge does not send it
			// again, and the commit it announced was never published.
			d.retryWebhookInBackground(evt)
			slog.Info("Webhook waits for room in the queue of the consumer",
				logfields.JobID(jobID),
				slog.String("forge", forgeName),
				slog.String("repo", repoFullName))
			return jobID
		}
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
	if d.orchestrationBus() == nil {
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

// runContext returns a context that is canceled when the daemon stops.
// The triggers use it: work that belongs to the daemon is canceled with
// it, and it does not write to the state of a daemon that is gone.
func (d *Daemon) runContext() context.Context {
	if d == nil {
		return context.Background()
	}

	d.lifecycle.mu.Lock()
	stopChan := d.lifecycle.stopChan
	d.lifecycle.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-stopChan:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}
