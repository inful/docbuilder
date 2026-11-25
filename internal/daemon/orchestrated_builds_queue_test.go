package daemon

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// A build that does not fit in the queue is not lost: the daemon waits
// for room.
func TestTheBuildWaitsForRoomInTheQueue(t *testing.T) {
	release := make(chan struct{})
	builder := &recordingBuilder{release: release}
	q := queue.NewBuildQueue(1, 1, builder)
	q.Start(t.Context())
	t.Cleanup(func() { q.Stop(context.Background()) })

	// The worker takes the first job and waits in the build.
	require.NoError(t, q.Enqueue(&queue.BuildJob{ID: "filler"}))
	require.Eventually(t, func() bool { return builder.built("filler") }, 2*time.Second, 10*time.Millisecond)
	// The queue is full now.
	require.NoError(t, q.Enqueue(&queue.BuildJob{ID: "second"}))

	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{
			Daemon:       &config.DaemonConfig{},
			Repositories: []config.Repository{{Name: "repo", URL: "https://example.invalid/repo.git"}},
		},
		BuildQueue: q,
	})

	done := make(chan struct{})
	go func() {
		d.enqueueOrchestratedBuild(t.Context(), events.BuildNow{JobID: "job-2"})
		close(done)
	}()

	// The queue is full: the build waits and is not lost.
	select {
	case <-done:
		t.Fatal("the build was dropped, the queue was full")
	case <-time.After(100 * time.Millisecond):
	}

	// The queue has room: the build runs.
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the build was not queued after the queue had room")
	}
	require.Eventually(t, func() bool { return builder.built("job-2") }, 5*time.Second, 10*time.Millisecond,
		"the build was not built")
}

// recordingBuilder records the jobs it builds. The first build waits
// until the test releases it.
type recordingBuilder struct {
	release chan struct{}

	mu       sync.Mutex
	builtIDs []string
}

func (b *recordingBuilder) Build(ctx context.Context, job *queue.BuildJob) (*models.BuildReport, error) {
	b.mu.Lock()
	b.builtIDs = append(b.builtIDs, job.ID)
	b.mu.Unlock()

	if b.release != nil {
		select {
		case <-b.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &models.BuildReport{}, nil
}

// built says whether the job was built.
func (b *recordingBuilder) built(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Contains(b.builtIDs, id)
}
