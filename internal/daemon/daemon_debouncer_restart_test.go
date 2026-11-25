package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The snapshot of the last run survives a restart: the first build
// after the restart pinned the commit of the last run, and the new
// content appeared one build late.
func TestTheSnapshotOfTheLastRunIsGoneAfterARestart(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:  time.Second,
		MaxDelay:     time.Second,
		PollInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx := t.Context()
	go func() { _ = debouncer.Run(ctx) }()
	<-debouncer.Ready()

	// A build is planned with the commit of the run.
	require.NoError(t, bus.Publish(ctx, events.BuildRequested{
		JobID:    "job-1",
		Snapshot: map[string]string{"https://example.invalid/repo.git": "OLD-SHA"},
	}))
	require.Eventually(t, func() bool {
		debouncer.mu.Lock()
		defer debouncer.mu.Unlock()
		return debouncer.snapshot != nil
	}, 2*time.Second, 10*time.Millisecond, "the build was not planned")

	// The daemon is started again: the pending build of the last run is
	// gone, and the commit with it.
	debouncer.SetBus(bus)

	debouncer.mu.Lock()
	defer debouncer.mu.Unlock()
	require.Nil(t, debouncer.snapshot, "the commit of the last run survives the restart")
	require.False(t, debouncer.pending, "the build of the last run survives the restart")
}
