package daemon

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// TestGetQueueLengthCountsWaitingJobs proves the reported queue length is
// the number of jobs actually waiting for a worker. The daemon used to keep
// a second counter that grew with every enqueue and was only corrected
// every 30 seconds, so the reported number drifted upwards.
func TestGetQueueLengthCountsWaitingJobs(t *testing.T) {
	d := &Daemon{build: &buildStack{}}
	if got := d.GetQueueLength(); got != 0 {
		t.Fatalf("expected 0 for a daemon without a build queue, got %d", got)
	}

	d.build.buildQueue = queue.NewBuildQueue(10, 0, &stubBuilder{})
	for _, id := range []string{"a", "b", "c"} {
		if err := d.build.buildQueue.Enqueue(&queue.BuildJob{ID: id}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	if got := d.GetQueueLength(); got != 3 {
		t.Fatalf("expected 3 waiting jobs, got %d", got)
	}
}

// stubBuilder satisfies queue.Builder without doing any work.
type stubBuilder struct{}

func (s *stubBuilder) Build(_ context.Context, _ *queue.BuildJob) (*models.BuildReport, error) {
	return &models.BuildReport{}, nil
}
