package daemon

import "context"

func (d *Daemon) goWorker(fn func()) {
	if d == nil || fn == nil {
		return
	}

	_ = d.workers.Go(fn)
}

func (d *Daemon) startWorkers(ctx context.Context) {
	if d == nil || ctx == nil {
		return
	}

	if d.orchestrationBus != nil {
		d.goWorker(func() { d.runBuildNowConsumer(ctx) })
		d.goWorker(func() { d.runWebhookReceivedConsumer(ctx) })
		d.goWorker(func() { d.runRepoRemovedConsumer(ctx) })
	}

	if d.buildDebouncer != nil {
		d.goWorker(func() { _ = d.buildDebouncer.Run(ctx) })
	}

	if d.repoUpdater != nil {
		d.goWorker(func() { d.repoUpdater.Run(ctx) })
	}
}
