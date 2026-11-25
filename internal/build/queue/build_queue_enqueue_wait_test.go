package queue

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// A job that does not fit in the queue waits for room: the build is not
// lost.
func TestEnqueueWaitWaitsForRoom(t *testing.T) {
	bq := NewBuildQueue(1, 1, &mockProcessJobBuilder{buildReport: &models.BuildReport{}})
	// No worker runs: the queue holds the one job.
	if err := bq.Enqueue(&BuildJob{ID: "first"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- bq.EnqueueWait(ctx, &BuildJob{ID: "second"}) }()

	select {
	case err := <-done:
		t.Fatalf("the job was queued, but the queue was full: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// The queue has room now: the job goes in.
	<-bq.jobs
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("enqueue wait: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the job was not queued after the queue had room")
	}
}

// A job that cannot be queued in time is lost, and the caller says so.
func TestEnqueueWaitGivesUpWhenTheTimeIsOver(t *testing.T) {
	bq := NewBuildQueue(1, 1, &mockProcessJobBuilder{buildReport: &models.BuildReport{}})
	if err := bq.Enqueue(&BuildJob{ID: "first"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	if err := bq.EnqueueWait(ctx, &BuildJob{ID: "second"}); err == nil {
		t.Fatal("the wait did not give up")
	}
}
