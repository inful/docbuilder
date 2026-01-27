package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

func TestDaemon_TriggerWebhookBuild_IgnoresIrrelevantPushChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cfg := &config.Config{
		Version: "2.0",
		Repositories: []config.Repository{
			{
				Name:   "org/repo",
				URL:    "https://gitlab.example.com/org/repo.git",
				Branch: "main",
				Paths:  []string{"docs"},
			},
		},
		Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}},
		Forges: []*config.ForgeConfig{{
			Name:    "forge-1",
			Type:    config.ForgeGitLab,
			BaseURL: "https://gitlab.example.com",
		}},
	}

	forgeManager := forge.NewForgeManager()
	forgeManager.AddForge(cfg.Forges[0], fakeForgeClient{})

	bus := events.NewBus()
	defer bus.Close()

	d := &Daemon{
		config:           cfg,
		stopChan:         make(chan struct{}),
		orchestrationBus: bus,
		forgeManager:     forgeManager,
		discovery:        forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache:   NewDiscoveryCache(),
	}
	d.status.Store(StatusRunning)

	repoUpdateCh, unsubRepoUpdate := events.Subscribe[events.RepoUpdateRequested](bus, 10)
	defer unsubRepoUpdate()

	go d.runWebhookReceivedConsumer(ctx)

	// Avoid flaky races where the webhook event is published before consumers subscribe.
	require.Eventually(t, func() bool {
		return events.SubscriberCount[events.WebhookReceived](bus) > 0
	}, 1*time.Second, 10*time.Millisecond)

	// Change outside docs path should not trigger a build.
	jobID := d.TriggerWebhookBuild("forge-1", "org/repo", "main", []string{"src/config.yaml"})
	require.NotEmpty(t, jobID)

	select {
	case <-repoUpdateCh:
		t.Fatal("expected no RepoUpdateRequested for non-docs change")
	case <-time.After(150 * time.Millisecond):
		// ok
	}

	// Change within docs path should request a repo update.
	jobID = d.TriggerWebhookBuild("forge-1", "org/repo", "main", []string{"docs/README.md"})
	require.NotEmpty(t, jobID)

	select {
	case got := <-repoUpdateCh:
		require.Equal(t, jobID, got.JobID)
		require.Equal(t, "https://gitlab.example.com/org/repo.git", got.RepoURL)
		require.Equal(t, "main", got.Branch)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for RepoUpdateRequested")
	}
}
