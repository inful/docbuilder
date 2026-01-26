package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
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
	buildCtx := t.Context()

	cfg := &config.Config{
		Version: "2.0",
		Daemon:  &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}},
		Forges: []*config.ForgeConfig{{
			Name:    "forge-1",
			Type:    config.ForgeForgejo,
			BaseURL: "https://git.home.luguber.info",
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

	d.discoveryCache.Update(&forge.DiscoveryResult{Repositories: []*forge.Repository{{
		Name:          "go-test-project",
		FullName:      "inful/go-test-project",
		CloneURL:      "https://git.home.luguber.info/inful/go-test-project.git",
		SSHURL:        "ssh://git@git.home.luguber.info/inful/go-test-project.git",
		DefaultBranch: "main",
		Metadata:      map[string]string{"forge_name": "forge-1"},
	}, {
		Name:          "other-project",
		FullName:      "inful/other-project",
		CloneURL:      "https://git.home.luguber.info/inful/other-project.git",
		SSHURL:        "ssh://git@git.home.luguber.info/inful/other-project.git",
		DefaultBranch: "main",
		Metadata:      map[string]string{"forge_name": "forge-1"},
	}}})

	jobID := d.TriggerWebhookBuild("inful/go-test-project", "main")
	require.NotEmpty(t, jobID)

	require.Eventually(t, func() bool {
		job, ok := d.buildQueue.JobSnapshot(jobID)
		if !ok {
			return false
		}
		return job.Status == queue.BuildStatusCompleted
	}, 2*time.Second, 10*time.Millisecond)

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
