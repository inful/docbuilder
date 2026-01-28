package daemon

import (
	"context"
	"log/slog"
)

func (d *Daemon) goWorker(name string, fn func()) {
	if d == nil || fn == nil {
		return
	}

	if ok := d.workers.Go(fn); !ok {
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

	if d.orchestrationBus != nil {
		d.goWorker("build_now_consumer", func() { d.runBuildNowConsumer(ctx) })
		d.goWorker("webhook_received_consumer", func() { d.runWebhookReceivedConsumer(ctx) })
		d.goWorker("repo_removed_consumer", func() { d.runRepoRemovedConsumer(ctx) })
	}

	if d.buildDebouncer != nil {
		d.goWorker("build_debouncer", func() { _ = d.buildDebouncer.Run(ctx) })
	}

	if d.repoUpdater != nil {
		d.goWorker("repo_updater", func() { d.repoUpdater.Run(ctx) })
	}
}
