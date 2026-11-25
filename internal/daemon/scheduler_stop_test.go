package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The stop of the scheduler respects the deadline of the context: a job
// that runs longer than that does not hold the daemon.
func TestTheSchedulerStopRespectsTheDeadline(t *testing.T) {
	s, err := NewScheduler()
	require.NoError(t, err)

	started := make(chan struct{})
	_, err = s.ScheduleEvery("slow", 10*time.Millisecond, func() {
		select {
		case <-started:
		default:
			close(started)
		}
		time.Sleep(2 * time.Second)
	})
	require.NoError(t, err)

	s.Start(t.Context())
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the job did not run")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	begin := time.Now()
	err = s.Stop(ctx)
	require.Error(t, err, "the stop must give up when the deadline is over")
	require.Less(t, time.Since(begin), time.Second, "the stop waited for the job")
}
