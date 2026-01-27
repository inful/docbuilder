package daemon

import (
	"context"
	"sync"
)

// WorkerGroup tracks daemon-owned goroutines and provides a safe shutdown
// boundary so we never call WaitGroup.Add concurrently with Wait.
type WorkerGroup struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	stopping bool
}

// Reset prepares the group for reuse after a full stop.
//
// This must only be called when all workers have already exited.
func (g *WorkerGroup) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.stopping = false
	g.wg = sync.WaitGroup{}
}

// Go starts a worker if the group is not stopping.
func (g *WorkerGroup) Go(fn func()) bool {
	if fn == nil {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopping {
		return false
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		fn()
	}()
	return true
}

// StopAndWait prevents new workers from being started and waits for all current
// workers to exit, bounded by ctx.
func (g *WorkerGroup) StopAndWait(ctx context.Context) error {
	g.mu.Lock()
	g.stopping = true
	g.mu.Unlock()

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
