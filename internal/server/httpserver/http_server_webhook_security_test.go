package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	handlers "git.home.luguber.info/inful/docbuilder/internal/server/handlers"
	"github.com/stretchr/testify/require"
)

// A forge webhook without a secret accepts any POST and starts a build.
// The daemon must refuse to start instead of serving an open trigger.
func TestWebhookMux_ForgeWithoutSecret_RefusesToStart(t *testing.T) {
	cfg := &config.Config{
		Forges: []*config.ForgeConfig{
			{
				Name: "company-github",
				Type: config.ForgeGitHub,
				Webhook: &config.WebhookConfig{
					Secret: "",
					Path:   "/webhooks/github",
				},
			},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{WebhookPort: 0, DocsPort: 0, AdminPort: 0},
		},
	}

	srv := New(cfg, &webhookRuntimeStub{}, Options{})

	mux, err := srv.webhookMux()
	require.Error(t, err)
	require.Nil(t, mux)
	require.Contains(t, err.Error(), "no secret")
	require.Contains(t, err.Error(), "company-github")
}

// A webhook path without a secret is refused; a forge without a webhook
// block at all is fine (no endpoint, nothing to protect).
func TestWebhookMux_ForgeWithoutWebhookBlock_IsAllowed(t *testing.T) {
	cfg := &config.Config{
		Forges: []*config.ForgeConfig{
			{Name: "company-github", Type: config.ForgeGitHub},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{WebhookPort: 0, DocsPort: 0, AdminPort: 0},
		},
	}

	srv := New(cfg, &webhookRuntimeStub{}, Options{})

	mux, err := srv.webhookMux()
	require.NoError(t, err)
	require.NotNil(t, mux)
}

// The webhook port is the one port that has to be reachable from the
// internet, so the body of a request must be bounded. The limit itself
// is exercised in the handlers package, where it lives.
func TestWebhookMux_OversizedBody_IsRejected(t *testing.T) {
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
		ForgeClients: map[string]forge.Client{forgeName: client},
		WebhookConfigs: map[string]*config.WebhookConfig{
			forgeName: cfg.Forges[0].Webhook,
		},
	})

	mux, err := srv.webhookMux()
	require.NoError(t, err)

	// One byte more than the limit.
	oversized := strings.Repeat("a", handlers.MaxWebhookBodyBytes+1)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, whPath, bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.NotEqual(t, http.StatusAccepted, rr.Code)
	require.False(t, runtime.called, "an oversized payload must not start a build")
}
