package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
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

func TestDaemon_currentReposForOrchestratedBuild_UsesCachedDiscoveryEvenWithError(t *testing.T) {
	cfg := &config.Config{}

	forgeManager := forge.NewForgeManager()
	d := &Daemon{
		config:         cfg,
		forgeManager:   forgeManager,
		discovery:      forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache: NewDiscoveryCache(),
	}

	d.discoveryCache.Update(&forge.DiscoveryResult{Repositories: []*forge.Repository{{
		Name:          "repo-1",
		FullName:      "org/repo-1",
		CloneURL:      "https://example.invalid/org/repo-1.git",
		DefaultBranch: "main",
		Metadata:      map[string]string{},
	}}})
	d.discoveryCache.SetError(errors.New("gitlab unavailable"))

	repos := d.currentReposForOrchestratedBuild()
	require.Len(t, repos, 1)
	require.Equal(t, "repo-1", repos[0].Name)
	require.Equal(t, "https://example.invalid/org/repo-1.git", repos[0].URL)
	require.Equal(t, "main", repos[0].Branch)
}

func TestDaemon_currentReposForOrchestratedBuild_ReturnsNilWhenDiscoveryMissing(t *testing.T) {
	cfg := &config.Config{}

	forgeManager := forge.NewForgeManager()
	d := &Daemon{
		config:         cfg,
		forgeManager:   forgeManager,
		discovery:      forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache: NewDiscoveryCache(),
	}

	d.discoveryCache.SetError(errors.New("gitlab unavailable"))

	repos := d.currentReposForOrchestratedBuild()
	require.Nil(t, repos)
}
