package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// A webhook whose signature cannot be verified must be rejected, not
// accepted: an endpoint that cannot check a request must not trigger a
// build.
func TestValidateWebhookSignature_FailsClosed(t *testing.T) {
	const forgeName = "github"

	tests := []struct {
		name           string
		webhookConfigs map[string]*config.WebhookConfig
		forgeClients   map[string]forge.Client
	}{
		{
			name:           "no webhook config at all",
			webhookConfigs: nil,
			forgeClients:   map[string]forge.Client{forgeName: forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub)},
		},
		{
			name:           "forge has no webhook config",
			webhookConfigs: map[string]*config.WebhookConfig{},
			forgeClients:   map[string]forge.Client{forgeName: forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub)},
		},
		{
			name: "forge has no secret",
			webhookConfigs: map[string]*config.WebhookConfig{
				forgeName: {Path: "/webhooks/github"},
			},
			forgeClients: map[string]forge.Client{forgeName: forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub)},
		},
		{
			name: "no forge client",
			webhookConfigs: map[string]*config.WebhookConfig{
				forgeName: {Secret: "s3cret"},
			},
			forgeClients: map[string]forge.Client{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &WebhookHandlers{
				webhookConfig: tc.webhookConfigs,
				forgeClients:  tc.forgeClients,
			}
			r := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader("{}"))
			err := h.validateWebhookSignature(forgeName, "X-Hub-Signature-256", []byte("{}"), r)
			if err == nil {
				t.Fatal("expected the webhook to be rejected, got no error")
			}
		})
	}
}

// A webhook with a valid signature is still accepted.
func TestValidateWebhookSignature_AcceptsValidSignature(t *testing.T) {
	const forgeName = "github"
	const secret = "s3cret"

	client := forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub).WithWebhookSecret(secret)
	h := &WebhookHandlers{
		webhookConfig: map[string]*config.WebhookConfig{
			forgeName: {Secret: secret},
		},
		forgeClients: map[string]forge.Client{forgeName: client},
	}

	body := []byte(`{"action":"opened"}`)
	r := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(string(body)))
	r.Header.Set("X-Hub-Signature-256", "sha256=valid-signature")

	if err := h.validateWebhookSignature(forgeName, "X-Hub-Signature-256", body, r); err != nil {
		t.Fatalf("expected the webhook to be accepted, got: %v", err)
	}
}
