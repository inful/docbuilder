package daemon

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	ferrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

const orchestrationPublishTimeout = 2 * time.Second

func publishOrchestrationEventOnBus(ctx context.Context, bus *events.Bus, evt any) error {
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}
	if bus == nil {
		return ferrors.ValidationError("bus cannot be nil").Build()
	}

	publishCtx, cancel := context.WithTimeout(ctx, orchestrationPublishTimeout)
	defer cancel()

	return bus.Publish(publishCtx, evt)
}

func (d *Daemon) publishOrchestrationEvent(ctx context.Context, evt any) error {
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}
	if d == nil || d.orchestrationBus == nil {
		return ferrors.DaemonError("orchestration bus not initialized").Build()
	}

	return publishOrchestrationEventOnBus(ctx, d.orchestrationBus, evt)
}
