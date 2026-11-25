package daemon

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The bus of the current run is written when the daemon starts and
// read by the goroutine of the debouncer: the pointer was read without
// a lock, and the race detector reported it.
func TestTheBusOfTheDebouncerIsReadUnderTheLock(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:  time.Millisecond,
		MaxDelay:     time.Millisecond,
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)

	ctx := t.Context()
	go func() { _ = debouncer.Run(ctx) }()
	<-debouncer.Ready()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			debouncer.SetBus(bus)
		}()
		go func() {
			defer wg.Done()
			_ = bus.Publish(ctx, events.BuildRequested{JobID: fmt.Sprintf("job-%d", i)})
		}()
	}
	wg.Wait()
}

// The bus of the current run is written when the daemon starts and
// read by the goroutine of the updater: the pointer was written
// without a lock, and the race detector reported it.
func TestTheBusOfTheRepoUpdaterIsReadUnderTheLock(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	updater := NewRepoUpdater(bus, nil, nil, nil)

	ctx := t.Context()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		updater.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		updater.SetBus(bus)
	}()
	wg.Wait()
}
