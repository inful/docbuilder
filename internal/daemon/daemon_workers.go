package daemon

import "context"

func (d *Daemon) startWorkers(ctx context.Context) {
	if d == nil || ctx == nil {
		return
	}

	if d.orchestrationBus != nil {
		d.workers.Add(1)
		go func() {
			defer d.workers.Done()
			d.runBuildNowConsumer(ctx)
		}()

		d.workers.Add(1)
		go func() {
			defer d.workers.Done()
			d.runWebhookReceivedConsumer(ctx)
		}()

		d.workers.Add(1)
		go func() {
			defer d.workers.Done()
			d.runRepoRemovedConsumer(ctx)
		}()
	}

	if d.buildDebouncer != nil {
		d.workers.Add(1)
		go func() {
			defer d.workers.Done()
			_ = d.buildDebouncer.Run(ctx)
		}()
	}

	if d.repoUpdater != nil {
		d.workers.Add(1)
		go func() {
			defer d.workers.Done()
			d.repoUpdater.Run(ctx)
		}()
	}
}
