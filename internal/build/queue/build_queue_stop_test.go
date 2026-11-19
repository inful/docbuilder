package queue

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// blockingBuilder starts a build and waits until the build is canceled.
type blockingBuilder struct {
	started  chan struct{}
	canceled chan struct{}
}

func (b *blockingBuilder) Build(ctx context.Context, _ *BuildJob) (*models.BuildReport, error) {
	close(b.started)
	<-ctx.Done()
	close(b.canceled)
	return nil, ctx.Err()
}

// Stop cancels the build that is running. The cancel function is written
// when the job starts and read when the queue stops, so both have to use
// the same lock - otherwise a shutdown during a build is a data race and
// the build can keep running.
func TestBuildQueue_StopCancelsTheRunningBuild(t *testing.T) {
	builder := &blockingBuilder{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}

	bq := NewBuildQueue(4, 1, builder)
	bq.Start(t.Context())

	job := &BuildJob{ID: "job-1", Type: BuildTypeManual, Priority: PriorityNormal, CreatedAt: time.Now()}
	if err := bq.Enqueue(job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-builder.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the build did not start")
	}

	bq.Stop(t.Context())

	select {
	case <-builder.canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not cancel the running build")
	}
}

// Stop can be called twice - a signal handler and a deferred shutdown -
// and closing a closed channel panics.
func TestBuildQueue_StopTwiceDoesNotPanic(t *testing.T) {
	bq := NewBuildQueue(4, 1, &mockProcessJobBuilder{})
	bq.Start(t.Context())

	bq.Stop(t.Context())
	bq.Stop(t.Context())
}
