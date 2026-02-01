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

func TestDaemon_Webhook_WhenNoReposYet_RetriesAfterDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	cfg := &config.Config{
		Version: "2.0",
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
	}
	d.status.Store(StatusRunning)

	reqCh, unsub := events.Subscribe[events.RepoUpdateRequested](bus, 10)
	defer unsub()

	go d.runWebhookReceivedConsumer(ctx)

	require.Eventually(t, func() bool {
		return events.SubscriberCount[events.WebhookReceived](bus) > 0
	}, 1*time.Second, 10*time.Millisecond)

	jobID := d.TriggerWebhookBuild("forge-1", "org/go-test-project", "main", []string{"docs/README.md"})
	require.NotEmpty(t, jobID)

	// Simulate discovery completing after the webhook arrives.
	time.AfterFunc(50*time.Millisecond, func() {
		d.discoveryCache.Update(&forge.DiscoveryResult{Repositories: []*forge.Repository{{
			Name:          "go-test-project",
			FullName:      "org/go-test-project",
			CloneURL:      "https://forgejo.example.com/org/go-test-project.git",
			SSHURL:        "ssh://git@forgejo.example.com/org/go-test-project.git",
			DefaultBranch: "main",
			Metadata:      map[string]string{"forge_name": "forge-1"},
		}}})
	})

	select {
	case got := <-reqCh:
		require.Equal(t, jobID, got.JobID)
		require.Equal(t, "https://forgejo.example.com/org/go-test-project.git", got.RepoURL)
		require.Equal(t, "main", got.Branch)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for RepoUpdateRequested after discovery")
	}
}
