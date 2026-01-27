package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"github.com/stretchr/testify/require"
)

type alwaysChangedRemoteHeadChecker struct{}

func (alwaysChangedRemoteHeadChecker) CheckRemoteChanged(_ *git.RemoteHeadCache, _ config.Repository, _ string) (bool, string, error) {
	return true, "deadbeef", nil
}

func TestDaemon_TriggerWebhookBuild_Orchestrated_EnqueuesWebhookJobWithBranchOverride(t *testing.T) {
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
		Name:          "go-test-project",
		FullName:      "org/go-test-project",
		CloneURL:      "https://forgejo.example.com/org/go-test-project.git",
		SSHURL:        "ssh://git@forgejo.example.com/org/go-test-project.git",
		DefaultBranch: "main",
		Metadata:      map[string]string{"forge_name": "forge-1"},
	}, {
		Name:          "other-project",
		FullName:      "org/other-project",
		CloneURL:      "https://forgejo.example.com/org/other-project.git",
		SSHURL:        "ssh://git@forgejo.example.com/org/other-project.git",
		DefaultBranch: "main",
		Metadata:      map[string]string{"forge_name": "forge-1"},
	}}})

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow: 200 * time.Millisecond,
		MaxDelay:    500 * time.Millisecond,
		CheckBuildRunning: func() bool {
			return len(bq.GetActiveJobs()) > 0
		},
		PollInterval: 5 * time.Millisecond,
	})
	require.NoError(t, err)
	d.buildDebouncer = debouncer

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	d.repoUpdater = NewRepoUpdater(bus, alwaysChangedRemoteHeadChecker{}, cache, d.currentReposForOrchestratedBuild)

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

	jobID := d.TriggerWebhookBuild("org/go-test-project", "feature-branch", nil)
	require.NotEmpty(t, jobID)

	require.Eventually(t, func() bool {
		job, ok := bq.JobSnapshot(jobID)
		return ok && job != nil && job.TypedMeta != nil && len(job.TypedMeta.Repositories) == 2
	}, 2*time.Second, 10*time.Millisecond)

	job, ok := bq.JobSnapshot(jobID)
	require.True(t, ok)
	require.NotNil(t, job)
	require.Equal(t, queue.BuildTypeWebhook, job.Type)
	require.NotNil(t, job.TypedMeta)
	require.Len(t, job.TypedMeta.Repositories, 2)

	var target *config.Repository
	for i := range job.TypedMeta.Repositories {
		r := &job.TypedMeta.Repositories[i]
		if r.Name == "go-test-project" {
			target = r
			break
		}
	}
	require.NotNil(t, target)
	require.Equal(t, "feature-branch", target.Branch)
}

func TestDaemon_TriggerWebhookBuild_Orchestrated_ReusesPlannedJobIDWhenBuildRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	bq := queue.NewBuildQueue(10, 1, noOpBuilder{})
	bq.Start(ctx)
	defer bq.Stop(context.Background())

	cfg := &config.Config{
		Version: "2.0",
		Repositories: []config.Repository{
			{
				Name:   "org/go-test-project",
				URL:    "https://forgejo.example.com/org/go-test-project.git",
				Branch: "main",
				Paths:  []string{"docs"},
			},
		},
	}

	d := &Daemon{
		config:           cfg,
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		buildQueue:       bq,
	}
	d.status.Store(StatusRunning)

	var running atomic.Bool
	running.Store(true)

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       200 * time.Millisecond,
		MaxDelay:          500 * time.Millisecond,
		CheckBuildRunning: running.Load,
		PollInterval:      5 * time.Millisecond,
	})
	require.NoError(t, err)
	d.buildDebouncer = debouncer

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	d.repoUpdater = NewRepoUpdater(bus, alwaysChangedRemoteHeadChecker{}, cache, d.currentReposForOrchestratedBuild)

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

	// Seed a pending (coalesced) build so PlannedJobID is available.
	require.NoError(t, bus.Publish(context.Background(), events.BuildRequested{JobID: "job-seeded", Reason: "seed"}))
	require.Eventually(t, func() bool {
		planned, ok := d.buildDebouncer.PlannedJobID()
		return ok && planned == "job-seeded"
	}, 250*time.Millisecond, 5*time.Millisecond)

	jobID1 := d.TriggerWebhookBuild("org/go-test-project", "main", nil)
	jobID2 := d.TriggerWebhookBuild("org/go-test-project", "main", nil)
	require.NotEmpty(t, jobID1)
	require.Equal(t, jobID1, jobID2)
	require.Equal(t, "job-seeded", jobID1)

	running.Store(false)

	require.Eventually(t, func() bool {
		job, ok := bq.JobSnapshot(jobID1)
		return ok && job != nil && job.Status == queue.BuildStatusCompleted
	}, 2*time.Second, 10*time.Millisecond)
}
