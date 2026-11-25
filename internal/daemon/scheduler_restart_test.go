package daemon

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The engine of the scheduler cannot be started again after it was shut
// down: the periodic work of the daemon stops for good after a restart.
func TestTheSchedulerRunsItsJobsAgainAfterARestart(t *testing.T) {
	s, err := NewScheduler()
	require.NoError(t, err)

	var runs atomic.Int32
	_, err = s.ScheduleEvery("job", 10*time.Millisecond, func() { runs.Add(1) })
	require.NoError(t, err)

	s.Start(t.Context())
	require.Eventually(t, func() bool { return runs.Load() > 0 }, 2*time.Second, 10*time.Millisecond,
		"the job of the scheduler did not run")
	require.NoError(t, s.Stop(t.Context()))

	// The daemon is started again: the scheduler gets a new engine and
	// its jobs are scheduled again.
	require.NoError(t, s.Reset())
	_, err = s.ScheduleEvery("job", 10*time.Millisecond, func() { runs.Add(1) })
	require.NoError(t, err)
	s.Start(t.Context())

	require.Eventually(t, func() bool { return runs.Load() > 1 }, 2*time.Second, 10*time.Millisecond,
		"the job of the scheduler did not run after the restart")
}
