package daemon

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	ferrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

const orchestrationPublishTimeout = 2 * time.Second

func publishOrchestrationEventOnBus(ctx context.Context, bus *events.Bus, evt any) error {
	return publishOrchestrationEventOnBusWithTimeout(ctx, bus, evt, orchestrationPublishTimeout)
}

// publishOrchestrationEventOnBusWithTimeout publishes an event with a
// budget of its own.
func publishOrchestrationEventOnBusWithTimeout(ctx context.Context, bus *events.Bus, evt any, timeout time.Duration) error {
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}
	if bus == nil {
		return ferrors.ValidationError("bus cannot be nil").Build()
	}

	publishCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return bus.Publish(publishCtx, evt)
}

func (d *Daemon) publishOrchestrationEvent(ctx context.Context, evt any) error {
	return d.publishOrchestrationEventWithTimeout(ctx, evt, orchestrationPublishTimeout)
}

// publishOrchestrationEventWithTimeout publishes an event with a
// budget of its own.
func (d *Daemon) publishOrchestrationEventWithTimeout(ctx context.Context, evt any, timeout time.Duration) error {
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}
	if d == nil {
		return ferrors.DaemonError("orchestration bus not initialized").Build()
	}

	bus := d.orchestrationBus()
	if bus == nil {
		return ferrors.DaemonError("orchestration bus not initialized").Build()
	}

	return publishOrchestrationEventOnBusWithTimeout(ctx, bus, evt, timeout)
}
