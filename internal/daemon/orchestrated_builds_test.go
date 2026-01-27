package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"github.com/stretchr/testify/require"
)

func TestOrchestration_DebouncedBuildEnqueuesJob(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	bq := queue.NewBuildQueue(10, 1, noOpBuilder{})
	bq.Start(ctx)
	defer bq.Stop(context.Background())

	d := &Daemon{
		config: &config.Config{Repositories: []config.Repository{{
			Name:   "repo-1",
			URL:    "https://example.invalid/repo-1.git",
			Branch: "main",
			Paths:  []string{"docs"},
		}}},
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		buildQueue:       bq,
	}
	d.status.Store(StatusRunning)

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow: 10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
		CheckBuildRunning: func() bool {
			return len(bq.GetActiveJobs()) > 0
		},
		PollInterval: 5 * time.Millisecond,
	})
	require.NoError(t, err)

	go d.runBuildNowConsumer(ctx)
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	require.NoError(t, bus.Publish(context.Background(), events.BuildRequested{
		JobID:  "job-1",
		Reason: "test",
	}))

	require.Eventually(t, func() bool {
		job, ok := bq.JobSnapshot("job-1")
		return ok && job != nil
	}, 500*time.Millisecond, 10*time.Millisecond)
}
