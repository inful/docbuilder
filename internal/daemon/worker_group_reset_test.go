package daemon

import (
	"context"
	"testing"
	"time"
)

// A worker that outlived the deadline of the stop must not take the
// process down. The wait group was replaced while the worker still
// held a reference to it: the worker called Done() on a group that
// was thrown away and the counter went below zero.
func TestTheWorkerGroupSurvivesAWorkerThatOutlivedTheStop(t *testing.T) {
	group := &WorkerGroup{}
	started := make(chan struct{})
	release := make(chan struct{})

	group.Go(func() {
		close(started)
		<-release
	})
	<-started

	// The stop gives up: the worker is still running.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := group.StopAndWait(ctx); err == nil {
		t.Fatal("the stop did not give up")
	}

	// The daemon starts again.
	group.Reset()

	// The worker ends: it must not panic on a wait group that was
	// thrown away.
	close(release)
	time.Sleep(200 * time.Millisecond)
}
