package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// A queue that runs again after a stop needs a new stop channel: the
// old one is closed, and the workers would return at once. The second
// run of a daemon never built anything.
func TestTheQueueThatRunsAgainBuilds(t *testing.T) {
	builder := &countingBuilder{}
	bq := NewBuildQueue(4, 1, builder)

	bq.Start(t.Context())
	if err := bq.Enqueue(&BuildJob{ID: "job-1", Type: BuildTypeManual, Priority: PriorityNormal, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitFor(t, func() bool { return builder.calls() == 1 })

	// The queue stops and runs again.
	bq.Stop(t.Context())
	bq.Start(t.Context())

	if err := bq.Enqueue(&BuildJob{ID: "job-2", Type: BuildTypeManual, Priority: PriorityNormal, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitFor(t, func() bool { return builder.calls() == 2 })
}

// countingBuilder counts the builds it was asked for.
type countingBuilder struct {
	mu    sync.Mutex
	count int
}

func (b *countingBuilder) Build(_ context.Context, _ *BuildJob) (*models.BuildReport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.count++
	return &models.BuildReport{Outcome: models.OutcomeSuccess}, nil
}

func (b *countingBuilder) calls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// waitFor waits for the condition to hold.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("the condition did not hold")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
