package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

type noOpBuilder struct{}

func (noOpBuilder) Build(context.Context, *queue.BuildJob) (*models.BuildReport, error) {
	return &models.BuildReport{}, nil
}

type fakeForgeClient struct{}

func (fakeForgeClient) GetType() forge.Type { return forge.TypeForgejo }
func (fakeForgeClient) GetName() string     { return "forge-1" }

func (fakeForgeClient) ListOrganizations(context.Context) ([]*forge.Organization, error) {
	return []*forge.Organization{}, nil
}

func (fakeForgeClient) ListRepositories(context.Context, []string) ([]*forge.Repository, error) {
	return []*forge.Repository{}, nil
}

func (fakeForgeClient) GetRepository(context.Context, string, string) (*forge.Repository, error) {
	return &forge.Repository{}, nil
}

func (fakeForgeClient) CheckDocumentation(context.Context, *forge.Repository) error { return nil }

func (fakeForgeClient) ValidateWebhook([]byte, string, string) bool { return true }
func (fakeForgeClient) ParseWebhookEvent([]byte, string) (*forge.WebhookEvent, error) {
	return &forge.WebhookEvent{}, nil
}

func (fakeForgeClient) RegisterWebhook(context.Context, *forge.Repository, string) error { return nil }
func (fakeForgeClient) GetEditURL(*forge.Repository, string, string) string              { return "" }

func TestDaemon_TriggerWebhookBuild_MatchesDiscoveredRepo(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

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
		forgeManager:     forgeManager,
		discovery:        forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache:   NewDiscoveryCache(),
		buildQueue:       queue.NewBuildQueue(10, 1, noOpBuilder{}),
	}
	d.status.Store(StatusRunning)

	d.buildQueue.Start(ctx)
	defer d.buildQueue.Stop(context.Background())

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
		QuietWindow: 50 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		CheckBuildRunning: func() bool {
			return false
		},
		PollInterval: 5 * time.Millisecond,
	})
	require.NoError(t, err)
	d.buildDebouncer = debouncer

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	d.repoUpdater = NewRepoUpdater(bus, fixedRemoteHeadChecker{changed: true, sha: "deadbeef"}, cache, d.currentReposForOrchestratedBuild)

	go d.runWebhookReceivedConsumer(ctx)
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

	// Avoid flaky races where the webhook event is published before consumers subscribe.
	require.Eventually(t, func() bool {
		return events.SubscriberCount[events.WebhookReceived](bus) > 0 &&
			events.SubscriberCount[events.BuildNow](bus) > 0
	}, 1*time.Second, 10*time.Millisecond)

	jobID := d.TriggerWebhookBuild("forge-1", "org/go-test-project", "main", nil)
	require.NotEmpty(t, jobID)

	require.Eventually(t, func() bool {
		job, ok := d.buildQueue.JobSnapshot(jobID)
		if !ok {
			return false
		}
		return job.Status == queue.BuildStatusCompleted
	}, 5*time.Second, 10*time.Millisecond)

	job, ok := d.buildQueue.JobSnapshot(jobID)
	require.True(t, ok)
	require.NotNil(t, job)
	require.Equal(t, queue.BuildTypeWebhook, job.Type)
	require.NotNil(t, job.TypedMeta)
	require.Len(t, job.TypedMeta.Repositories, 2)

	// Target repo should be present and use the webhook branch.
	var target *config.Repository
	for i := range job.TypedMeta.Repositories {
		r := &job.TypedMeta.Repositories[i]
		if r.Name == "go-test-project" {
			target = r
			break
		}
	}
	require.NotNil(t, target)
	require.Equal(t, "main", target.Branch)
}
