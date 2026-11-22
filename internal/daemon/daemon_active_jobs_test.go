package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"github.com/stretchr/testify/require"
)

// blockingBuilder holds the build open until the test releases it, so
// the queue keeps the job in its active set.
type blockingBuilder struct {
	release chan struct{}
	started chan struct{}
}

func (b *blockingBuilder) Build(ctx context.Context, _ *queue.BuildJob) (*models.BuildReport, error) {
	close(b.started)
	select {
	case <-b.release:
	case <-ctx.Done():
	}
	return &models.BuildReport{}, nil
}

// The daemon reports the number of running builds. The counter used to
// track manual discovery runs, so a build in flight did not show up.
func TestDaemon_GetActiveJobsCountsRunningBuilds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	builder := &blockingBuilder{release: make(chan struct{}), started: make(chan struct{})}
	bq := queue.NewBuildQueue(10, 1, builder)
	bq.Start(ctx)
	defer bq.Stop(context.Background())

	d := daemonForTest(t, daemonForTestOpts{
		Config:     &config.Config{},
		BuildQueue: bq,
	})

	require.Equal(t, 0, d.GetActiveJobs(), "no build is running yet")

	require.NoError(t, bq.Enqueue(&queue.BuildJob{ID: "job-1", Type: queue.BuildTypeManual}))

	select {
	case <-builder.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the build to start")
	}

	require.Eventually(t, func() bool {
		return d.GetActiveJobs() == 1
	}, time.Second, 10*time.Millisecond, "the running build should be counted")

	close(builder.release)
	require.Eventually(t, func() bool {
		return d.GetActiveJobs() == 0
	}, time.Second, 10*time.Millisecond, "the finished build should no longer be counted")
}
