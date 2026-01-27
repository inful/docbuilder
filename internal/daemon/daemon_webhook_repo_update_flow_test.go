package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"github.com/stretchr/testify/require"
)

type fixedRemoteHeadChecker struct {
	changed bool
	sha     string
	err     error
}

func (f fixedRemoteHeadChecker) CheckRemoteChanged(_ *git.RemoteHeadCache, _ config.Repository, _ string) (bool, string, error) {
	return f.changed, f.sha, f.err
}

func TestDaemon_WebhookRepoUpdateFlow_RemoteChanged_EnqueuesBuild(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	bq := queue.NewBuildQueue(10, 1, noOpBuilder{})
	bq.Start(ctx)
	defer bq.Stop(context.Background())

	cfg := &config.Config{
		Version: "2.0",
		Repositories: []config.Repository{{
			Name:   "org/repo",
			URL:    "https://forgejo.example.com/org/repo.git",
			Branch: "main",
			Paths:  []string{"docs"},
		}},
	}

	d := &Daemon{
		config:           cfg,
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		buildQueue:       bq,
	}
	d.status.Store(StatusRunning)

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow: 50 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		CheckBuildRunning: func() bool {
			return len(bq.GetActiveJobs()) > 0
		},
		PollInterval: 5 * time.Millisecond,
	})
	require.NoError(t, err)
	d.buildDebouncer = debouncer

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	d.repoUpdater = NewRepoUpdater(bus, fixedRemoteHeadChecker{changed: true, sha: "deadbeef"}, cache, d.currentReposForOrchestratedBuild)

	repoUpdatedCh, unsubRepoUpdated := events.Subscribe[events.RepoUpdated](bus, 10)
	defer unsubRepoUpdated()

	go d.runBuildNowConsumer(ctx)
	go d.repoUpdater.Run(ctx)
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-d.repoUpdater.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for repo updater ready")
	}
	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	jobID := d.TriggerWebhookBuild("org/repo", "main", nil)
	require.NotEmpty(t, jobID)

	select {
	case got := <-repoUpdatedCh:
		require.Equal(t, jobID, got.JobID)
		require.True(t, got.Changed)
		require.Equal(t, "deadbeef", got.CommitSHA)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for RepoUpdated")
	}

	require.Eventually(t, func() bool {
		job, ok := bq.JobSnapshot(jobID)
		return ok && job != nil && job.Status == queue.BuildStatusCompleted
	}, 2*time.Second, 10*time.Millisecond)
}

func TestDaemon_WebhookRepoUpdateFlow_RemoteUnchanged_DoesNotEnqueueBuild(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	bq := queue.NewBuildQueue(10, 1, noOpBuilder{})
	bq.Start(ctx)
	defer bq.Stop(context.Background())

	cfg := &config.Config{
		Version: "2.0",
		Repositories: []config.Repository{{
			Name:   "org/repo",
			URL:    "https://forgejo.example.com/org/repo.git",
			Branch: "main",
			Paths:  []string{"docs"},
		}},
	}

	d := &Daemon{
		config:           cfg,
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		buildQueue:       bq,
	}
	d.status.Store(StatusRunning)

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       50 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		CheckBuildRunning: func() bool { return false },
		PollInterval:      5 * time.Millisecond,
	})
	require.NoError(t, err)
	d.buildDebouncer = debouncer

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	d.repoUpdater = NewRepoUpdater(bus, fixedRemoteHeadChecker{changed: false, sha: "deadbeef"}, cache, d.currentReposForOrchestratedBuild)

	repoUpdatedCh, unsubRepoUpdated := events.Subscribe[events.RepoUpdated](bus, 10)
	defer unsubRepoUpdated()

	buildRequestedCh, unsubBuildRequested := events.Subscribe[events.BuildRequested](bus, 10)
	defer unsubBuildRequested()

	go d.runBuildNowConsumer(ctx)
	go d.repoUpdater.Run(ctx)
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-d.repoUpdater.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for repo updater ready")
	}
	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	jobID := d.TriggerWebhookBuild("org/repo", "main", nil)
	require.NotEmpty(t, jobID)

	select {
	case got := <-repoUpdatedCh:
		require.Equal(t, jobID, got.JobID)
		require.False(t, got.Changed)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for RepoUpdated")
	}

	select {
	case <-buildRequestedCh:
		t.Fatal("expected no BuildRequested when repo unchanged")
	case <-time.After(150 * time.Millisecond):
		// ok
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, ok := bq.JobSnapshot(jobID); ok {
			t.Fatalf("expected no job enqueued for %s", jobID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDaemon_WebhookRepoUpdateFlow_DiscoveryMode_RemoteUnchanged_DoesNotEnqueueBuild(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	bq := queue.NewBuildQueue(10, 1, noOpBuilder{})
	bq.Start(ctx)
	defer bq.Stop(context.Background())

	cfg := &config.Config{
		Version: "2.0",
		Daemon:  &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}},
		Forges: []*config.ForgeConfig{{
			Name:    "forge-1",
			Type:    config.ForgeForgejo,
			BaseURL: "https://forgejo.example.com",
		}},
	}

	forgeManager := forge.NewForgeManager()
	forgeManager.AddForge(cfg.Forges[0], fakeForgeClient{})

	d := &Daemon{
		config:           cfg,
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		buildQueue:       bq,
		forgeManager:     forgeManager,
		discovery:        forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache:   NewDiscoveryCache(),
	}
	d.status.Store(StatusRunning)

	d.discoveryCache.Update(&forge.DiscoveryResult{Repositories: []*forge.Repository{{
		Name:          "repo",
		FullName:      "org/repo",
		CloneURL:      "https://forgejo.example.com/org/repo.git",
		SSHURL:        "ssh://git@forgejo.example.com/org/repo.git",
		DefaultBranch: "main",
		Metadata:      map[string]string{"forge_name": "forge-1"},
	}}})

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       50 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		CheckBuildRunning: func() bool { return false },
		PollInterval:      5 * time.Millisecond,
	})
	require.NoError(t, err)
	d.buildDebouncer = debouncer

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	d.repoUpdater = NewRepoUpdater(bus, fixedRemoteHeadChecker{changed: false, sha: "deadbeef"}, cache, d.currentReposForOrchestratedBuild)

	repoUpdatedCh, unsubRepoUpdated := events.Subscribe[events.RepoUpdated](bus, 10)
	defer unsubRepoUpdated()

	buildRequestedCh, unsubBuildRequested := events.Subscribe[events.BuildRequested](bus, 10)
	defer unsubBuildRequested()

	go d.runBuildNowConsumer(ctx)
	go d.repoUpdater.Run(ctx)
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-d.repoUpdater.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for repo updater ready")
	}
	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	jobID := d.TriggerWebhookBuild("org/repo", "main", nil)
	require.NotEmpty(t, jobID)

	select {
	case got := <-repoUpdatedCh:
		require.Equal(t, jobID, got.JobID)
		require.False(t, got.Changed)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for RepoUpdated")
	}

	select {
	case <-buildRequestedCh:
		t.Fatal("expected no BuildRequested when repo unchanged")
	case <-time.After(150 * time.Millisecond):
		// ok
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, ok := bq.JobSnapshot(jobID); ok {
			t.Fatalf("expected no job enqueued for %s", jobID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
