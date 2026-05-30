package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

type webhookProbeForgeClient struct {
	repo                    forge.Repository
	hasDocs                 bool
	hasDocIgnore            bool
	getRepositoryCalls      atomic.Int32
	checkDocumentationCalls atomic.Int32
}

func (c *webhookProbeForgeClient) GetType() forge.Type { return forge.TypeForgejo }
func (c *webhookProbeForgeClient) GetName() string     { return "forge-1" }

func (c *webhookProbeForgeClient) ListOrganizations(context.Context) ([]*forge.Organization, error) {
	return []*forge.Organization{}, nil
}

func (c *webhookProbeForgeClient) ListRepositories(context.Context, []string) ([]*forge.Repository, error) {
	return []*forge.Repository{}, nil
}

func (c *webhookProbeForgeClient) GetRepository(_ context.Context, owner, repo string) (*forge.Repository, error) {
	c.getRepositoryCalls.Add(1)

	repoCopy := c.repo
	return &repoCopy, nil
}

func (c *webhookProbeForgeClient) CheckDocumentation(_ context.Context, repo *forge.Repository) error {
	c.checkDocumentationCalls.Add(1)
	repo.HasDocs = c.hasDocs
	repo.HasDocIgnore = c.hasDocIgnore
	return nil
}

func (c *webhookProbeForgeClient) ValidateWebhook([]byte, string, string) bool { return true }
func (c *webhookProbeForgeClient) ParseWebhookEvent([]byte, string) (*forge.WebhookEvent, error) {
	return &forge.WebhookEvent{}, nil
}

func (c *webhookProbeForgeClient) RegisterWebhook(context.Context, *forge.Repository, string) error {
	return nil
}

func (c *webhookProbeForgeClient) GetEditURL(*forge.Repository, string, string) string { return "" }

func TestDaemon_TriggerWebhookBuild_FetchesPreviouslyFilteredRepoWhenDocsAppear(t *testing.T) {
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
		Filtering: &config.FilteringConfig{
			RequiredPaths: []string{"docs"},
		},
		Forges: []*config.ForgeConfig{{
			Name:    "forge-1",
			Type:    config.ForgeForgejo,
			BaseURL: "https://forgejo.example.com",
		}},
	}

	forgeClient := &webhookProbeForgeClient{
		repo: forge.Repository{
			ID:            "2",
			Name:          "new-docs",
			FullName:      "org/new-docs",
			CloneURL:      "https://forgejo.example.com/org/new-docs.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/new-docs.git",
			DefaultBranch: "main",
			Metadata:      map[string]string{"forge_name": "forge-1"},
		},
		hasDocs: true,
	}

	forgeManager := forge.NewForgeManager()
	forgeManager.AddForge(cfg.Forges[0], forgeClient)

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

	d.discoveryCache.Update(&forge.DiscoveryResult{
		Repositories: []*forge.Repository{{
			ID:            "1",
			Name:          "existing-project",
			FullName:      "org/existing-project",
			CloneURL:      "https://forgejo.example.com/org/existing-project.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/existing-project.git",
			DefaultBranch: "main",
			Metadata:      map[string]string{"forge_name": "forge-1"},
		}},
		Filtered: []*forge.Repository{{
			ID:            "2",
			Name:          "new-docs",
			FullName:      "org/new-docs",
			CloneURL:      "https://forgejo.example.com/org/new-docs.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/new-docs.git",
			DefaultBranch: "main",
			HasDocs:       false,
			Metadata:      map[string]string{"forge_name": "forge-1"},
		}},
	})

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
	d.repoUpdater = NewRepoUpdater(bus, alwaysChangedRemoteHeadChecker{}, cache, d.currentReposForOrchestratedBuild)

	go d.runBuildNowConsumer(ctx)
	go d.runWebhookReceivedConsumer(ctx)
	go d.repoUpdater.Run(ctx)
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-d.repoUpdater.Ready():
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for repo updater ready")
	}

	select {
	case <-debouncer.Ready():
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for debouncer ready")
	}

	require.Eventually(t, func() bool {
		return events.SubscriberCount[events.WebhookReceived](bus) > 0 &&
			events.SubscriberCount[events.BuildNow](bus) > 0
	}, 1*time.Second, 10*time.Millisecond)

	jobID := d.TriggerWebhookBuild("forge-1", "org/new-docs", "main", []string{"docs/README.md"})
	require.NotEmpty(t, jobID)

	require.Eventually(t, func() bool {
		job, ok := bq.JobSnapshot(jobID)
		return ok && job != nil && job.Status == queue.BuildStatusCompleted
	}, 5*time.Second, 10*time.Millisecond)

	job, ok := bq.JobSnapshot(jobID)
	require.True(t, ok)
	require.NotNil(t, job)
	require.NotNil(t, job.TypedMeta)
	require.Len(t, job.TypedMeta.Repositories, 2)

	var target *config.Repository
	for i := range job.TypedMeta.Repositories {
		repo := &job.TypedMeta.Repositories[i]
		if repo.Name == "new-docs" {
			target = repo
			break
		}
	}
	require.NotNil(t, target)
	require.Equal(t, "main", target.Branch)
	require.Equal(t, "https://forgejo.example.com/org/new-docs.git", target.URL)
	require.EqualValues(t, 1, forgeClient.getRepositoryCalls.Load())
	require.EqualValues(t, 1, forgeClient.checkDocumentationCalls.Load())
}

func TestDaemon_TriggerWebhookBuild_DoesNotReviveDocIgnoredRepo(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	cfg := &config.Config{
		Version: "2.0",
		Daemon:  &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}},
		Filtering: &config.FilteringConfig{
			RequiredPaths: []string{"docs"},
		},
		Forges: []*config.ForgeConfig{{
			Name:    "forge-1",
			Type:    config.ForgeForgejo,
			BaseURL: "https://forgejo.example.com",
		}},
	}

	forgeClient := &webhookProbeForgeClient{
		repo: forge.Repository{
			ID:            "2",
			Name:          "new-docs",
			FullName:      "org/new-docs",
			CloneURL:      "https://forgejo.example.com/org/new-docs.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/new-docs.git",
			DefaultBranch: "main",
			Metadata:      map[string]string{"forge_name": "forge-1"},
		},
		hasDocs:      true,
		hasDocIgnore: true,
	}

	forgeManager := forge.NewForgeManager()
	forgeManager.AddForge(cfg.Forges[0], forgeClient)

	d := &Daemon{
		config:           cfg,
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		forgeManager:     forgeManager,
		discovery:        forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache:   NewDiscoveryCache(),
	}
	d.status.Store(StatusRunning)

	d.discoveryCache.Update(&forge.DiscoveryResult{
		Repositories: []*forge.Repository{{
			ID:            "1",
			Name:          "existing-project",
			FullName:      "org/existing-project",
			CloneURL:      "https://forgejo.example.com/org/existing-project.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/existing-project.git",
			DefaultBranch: "main",
			Metadata:      map[string]string{"forge_name": "forge-1"},
		}},
		Filtered: []*forge.Repository{{
			ID:            "2",
			Name:          "new-docs",
			FullName:      "org/new-docs",
			CloneURL:      "https://forgejo.example.com/org/new-docs.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/new-docs.git",
			DefaultBranch: "main",
			HasDocs:       false,
			Metadata:      map[string]string{"forge_name": "forge-1"},
		}},
	})

	repoUpdateCh, unsubRepoUpdate := events.Subscribe[events.RepoUpdateRequested](bus, 10)
	defer unsubRepoUpdate()

	go d.runWebhookReceivedConsumer(ctx)

	require.Eventually(t, func() bool {
		return events.SubscriberCount[events.WebhookReceived](bus) > 0
	}, 1*time.Second, 10*time.Millisecond)

	jobID := d.TriggerWebhookBuild("forge-1", "org/new-docs", "main", []string{"docs/README.md"})
	require.NotEmpty(t, jobID)

	select {
	case <-repoUpdateCh:
		t.Fatal("expected no RepoUpdateRequested for .docignore-protected repo")
	case <-time.After(200 * time.Millisecond):
	}

	require.EqualValues(t, 1, forgeClient.getRepositoryCalls.Load())
	require.EqualValues(t, 1, forgeClient.checkDocumentationCalls.Load())
	require.Len(t, d.currentReposForOrchestratedBuild(), 1)
}
