package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

func TestDaemon_TriggerWebhookBuild_IgnoresIrrelevantPushChanges(t *testing.T) {
	buildCtx := t.Context()

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

	d := &Daemon{
		config:         cfg,
		stopChan:       make(chan struct{}),
		forgeManager:   forgeManager,
		discovery:      forge.NewDiscoveryService(forgeManager, cfg.Filtering),
		discoveryCache: NewDiscoveryCache(),
		buildQueue:     queue.NewBuildQueue(10, 1, noOpBuilder{}),
	}
	d.status.Store(StatusRunning)

	d.buildQueue.Start(buildCtx)
	defer d.buildQueue.Stop(context.Background())

	// Change outside docs path should not trigger a build.
	jobID := d.TriggerWebhookBuild("org/repo", "main", []string{"src/config.yaml"})
	require.Empty(t, jobID)

	// Change within docs path should trigger a build.
	jobID = d.TriggerWebhookBuild("org/repo", "main", []string{"docs/README.md"})
	require.NotEmpty(t, jobID)

	require.Eventually(t, func() bool {
		job, ok := d.buildQueue.JobSnapshot(jobID)
		if !ok {
			return false
		}
		return job.Status == queue.BuildStatusCompleted
	}, 2*time.Second, 10*time.Millisecond)
}
