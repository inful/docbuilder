package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The bus has no queue: an event that is published before the consumer
// is subscribed is lost. The debouncer and the updater subscribed
// inside their own goroutine, and the request that was published in
// the window between the start of the daemon and the subscription of
// the worker was lost.
func TestTheDebouncerAndTheUpdaterAreSubscribedWhenTheWorkersStart(t *testing.T) {
	for range 20 {
		bus := events.NewBus()

		debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
			QuietWindow:  time.Millisecond,
			MaxDelay:     time.Millisecond,
			PollInterval: time.Millisecond,
		})
		require.NoError(t, err)

		updater := NewRepoUpdater(bus, alwaysChangedRemoteHeadChecker{}, nil, nil)

		d := daemonForTest(t, daemonForTestOpts{
			Config:           &config.Config{Daemon: &config.DaemonConfig{}},
			OrchestrationBus: bus,
			BuildDebouncer:   debouncer,
			RepoUpdater:      updater,
		})

		d.startWorkers(t.Context())

		require.Equal(t, 1, events.SubscriberCount[events.BuildRequested](bus),
			"the debouncer is not subscribed to the bus of the run")
		require.Equal(t, 1, events.SubscriberCount[events.RepoUpdateRequested](bus),
			"the updater is not subscribed to the bus of the run")

		bus.Close()
	}
}
