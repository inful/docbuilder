package daemon

import (
	"context"
	"time"

	ferrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

const orchestrationPublishTimeout = 2 * time.Second

func (d *Daemon) publishOrchestrationEvent(ctx context.Context, evt any) error {
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}
	if d == nil || d.orchestrationBus == nil {
		return ferrors.DaemonError("orchestration bus not initialized").Build()
	}

	publishCtx, cancel := context.WithTimeout(ctx, orchestrationPublishTimeout)
	defer cancel()

	return d.orchestrationBus.Publish(publishCtx, evt)
}
