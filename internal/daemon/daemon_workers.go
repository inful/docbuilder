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
		d.goWorker("webhook_received_consumer", func() { d.runWebhookReceivedConsumer(ctx) })
		d.goWorker("repo_removed_consumer", func() { d.runRepoRemovedConsumer(ctx) })
	}

	if d.build.buildDebouncer != nil {
		d.goWorker("build_debouncer", func() { _ = d.build.buildDebouncer.Run(ctx) })
	}

	if d.build.repoUpdater != nil {
		d.goWorker("repo_updater", func() { d.build.repoUpdater.Run(ctx) })
	}
}
