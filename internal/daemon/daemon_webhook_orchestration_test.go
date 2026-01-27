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
		Repositories: []config.Repository{
			{
				Name:   "org/go-test-project",
				URL:    "https://forgejo.example.com/org/go-test-project.git",
				Branch: "main",
				Paths:  []string{"docs"},
			},
			{
				Name:   "org/other-project",
				URL:    "https://forgejo.example.com/org/other-project.git",
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

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow: 200 * time.Millisecond,
		MaxDelay:    500 * time.Millisecond,
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

	jobID := d.TriggerWebhookBuild("org/go-test-project", "feature-branch", nil)
	require.NotEmpty(t, jobID)

	require.Eventually(t, func() bool {
		job, ok := bq.JobSnapshot(jobID)
		return ok && job != nil && job.Status == queue.BuildStatusCompleted
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
		if r.Name == "org/go-test-project" {
			target = r
			break
		}
	}
	require.NotNil(t, target)
	require.Equal(t, "feature-branch", target.Branch)
}
