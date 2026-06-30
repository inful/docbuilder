package lifecycle

import "context"

// StopAwareContext returns a context that is canceled when either the parent
// context is done or the supplied stop channel is closed.
//
// This preserves the historical behavior where closing the stop channel
// unblocks in-flight work even when the caller passes context.Background().
//
// Callers MUST call the returned cancel func when the derived context is no
// longer needed; otherwise the internal stop-listener goroutine may live for
// the lifetime of the parent context.
func StopAwareContext(parent context.Context, stop <-chan struct{}) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if stop == nil {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel
	}

	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
			// parent canceled; nothing else to do
		}
	}()
	return ctx, cancel
}
