package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The counters of the builds read zero: the queue emitted the events
// through the emitter of the state, and nobody called the daemon. The
// dashboard showed a daemon that never built anything.
func TestTheBuildsOfTheQueueAreCounted(t *testing.T) {
	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})

	d.serve.metrics = NewMetricsCollector()

	bq := queue.NewBuildQueue(4, 1, &stubBuilder{})
	d.build.buildQueue = bq
	d.wireBuildEvents()

	bq.Start(t.Context())
	t.Cleanup(func() { bq.Stop(t.Context()) })

	require.NoError(t, bq.Enqueue(&queue.BuildJob{ID: "job-1", Type: queue.BuildTypeManual}))

	require.Eventually(t, func() bool {
		snap := d.serve.metrics.GetSnapshot()
		return snap.Counters["build_completed_total"] > 0
	}, 2*time.Second, 10*time.Millisecond, "the build was not counted")
}
