package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// stubbornBuilder ignores the cancellation of its context: it stands in for
// a clone to a host that never answers.
type stubbornBuilder struct {
	started chan struct{}
	release chan struct{}
}

func (b *stubbornBuilder) Build(_ context.Context, _ *BuildJob) (*models.BuildReport, error) {
	close(b.started)
	<-b.release
	return nil, errAbandoned
}

// errAbandoned is the error of a build that was given up on.
var errAbandoned = errors.New("the build was abandoned")

// The stop of the queue is bounded by its context. A build that does not
// stop must not keep the daemon from shutting down: the daemon writes its
// state file after the queue is stopped.
func TestBuildQueue_StopGivesUpWhenTheBuildDoesNotStop(t *testing.T) {
	builder := &stubbornBuilder{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	defer close(builder.release)

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

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		bq.Stop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return when the deadline passed")
	}
}
