package daemon

import (
	"context"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

func (d *Daemon) goWorker(name string, fn func()) {
	if d == nil || fn == nil {
		return
	}

	if ok := d.lifecycle.workers.Go(fn); !ok {
		if name == "" {
			name = "(unnamed)"
		}
		slog.Warn("Worker not started (daemon stopping)", slog.String("worker", name))
	}
}

func (d *Daemon) startWorkers(ctx context.Context) {
	if d == nil || ctx == nil {
		return
	}

	if d.serve.orchestrationBus != nil {
		// The subscription of the build-now events happens here and not
		// in the goroutine: the bus has no queue, and an event that is
		// published before the consumer is subscribed is lost.
		buildNowCh, unsubscribe := events.Subscribe[events.BuildNow](d.serve.orchestrationBus, 16)
		d.goWorker("build_now_consumer", func() { d.consumeBuildNow(ctx, buildNowCh, unsubscribe) })

		// The subscription of the webhooks happens here and not in the
		// goroutine: the server accepts the requests before the
		// consumers are subscribed, and a webhook that arrives in that
		// window is lost.
		webhookCh, unsubscribeWebhook := events.Subscribe[events.WebhookReceived](d.serve.orchestrationBus, 32)
		d.goWorker("webhook_received_consumer", func() { d.consumeWebhookReceived(ctx, webhookCh, unsubscribeWebhook) })

		// The removal of a repository is published by the discovery:
		// the event is lost when nobody is subscribed to it.
		repoRemovedCh, unsubscribeRepoRemoved := events.Subscribe[events.RepoRemoved](d.serve.orchestrationBus, 16)
		d.goWorker("repo_removed_consumer", func() { d.consumeRepoRemoved(ctx, repoRemovedCh, unsubscribeRepoRemoved) })
	}

	// The subscription of the debouncer and of the updater happens here
	// and not in the goroutine: the bus has no queue, and a request
	// that is published before the consumer is subscribed is lost.
	if d.build.buildDebouncer != nil {
		d.build.buildDebouncer.Subscribe()
		d.goWorker("build_debouncer", func() { _ = d.build.buildDebouncer.Run(ctx) })
	}

	if d.build.repoUpdater != nil {
		d.build.repoUpdater.Subscribe()
		d.goWorker("repo_updater", func() { d.build.repoUpdater.Run(ctx) })
	}
}
