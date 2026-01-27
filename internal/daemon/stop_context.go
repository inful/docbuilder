package daemon

import (
	"context"
)

// stopAwareContext returns a context that is canceled when either the parent
// context is done or the daemon stop channel is closed.
//
// This preserves the historical behavior where closing stopChan unblocks
// in-flight work even when the caller passes context.Background().
func (d *Daemon) stopAwareContext(parent context.Context) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	if d == nil || d.stopChan == nil {
		return parent
	}

	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-d.stopChan:
			cancel()
		case <-ctx.Done():
			// parent canceled; nothing else to do
		}
	}()
	return ctx
}
