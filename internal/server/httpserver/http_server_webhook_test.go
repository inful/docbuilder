package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"github.com/stretchr/testify/require"
)

type webhookRuntimeStub struct {
	called bool
	forge  string
	repo   string
	branch string
}

func (r *webhookRuntimeStub) GetStatus() string             { return "running" }
func (r *webhookRuntimeStub) GetActiveJobs() int            { return 0 }
func (r *webhookRuntimeStub) GetStartTime() time.Time       { return time.Unix(0, 0) }
func (r *webhookRuntimeStub) HTTPRequestsTotal() int        { return 0 }
func (r *webhookRuntimeStub) RepositoriesTotal() int        { return 0 }
func (r *webhookRuntimeStub) LastDiscoveryDurationSec() int { return 0 }
func (r *webhookRuntimeStub) LastBuildDurationSec() int     { return 0 }
func (r *webhookRuntimeStub) TriggerDiscovery() string      { return "" }
func (r *webhookRuntimeStub) TriggerBuild() string          { return "" }
func (r *webhookRuntimeStub) GetQueueLength() int           { return 0 }

func (r *webhookRuntimeStub) TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string {
	r.called = true
	r.forge = forgeName
	r.repo = repoFullName
	r.branch = branch
	return "job-123"
}

func TestWebhookMux_ConfiguredForgePath_TriggersBuild(t *testing.T) {
	ctx := context.Background()

	forgeName := "company-github"
	whSecret := "test-secret"
	whPath := "/webhooks/github"

	cfg := &config.Config{
		Forges: []*config.ForgeConfig{
			{
				Name: forgeName,
				Type: config.ForgeGitHub,
				Webhook: &config.WebhookConfig{
					Secret: whSecret,
					Path:   whPath,
					Events: []string{"push"},
				},
			},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{WebhookPort: 0, DocsPort: 0, AdminPort: 0},
		},
	}

	client := forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub).WithWebhookSecret(whSecret)

	runtime := &webhookRuntimeStub{}
	srv := New(cfg, runtime, Options{
		ForgeClients: map[string]forge.Client{
			forgeName: client,
		},
		WebhookConfigs: map[string]*config.WebhookConfig{
			forgeName: cfg.Forges[0].Webhook,
		},
	})

	mux, err := srv.webhookMux()
	require.NoError(t, err)
	require.NotNil(t, mux)

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, whPath, bytes.NewBufferString(`{"hello":"world"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=valid-signature")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Code)
	require.True(t, runtime.called)
	require.Equal(t, forgeName, runtime.forge)
	require.Equal(t, "test-org/mock-repo", runtime.repo)
	require.Equal(t, "main", runtime.branch)
}
