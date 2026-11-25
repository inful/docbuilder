package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// A body that is too large is not a bad request: the sender sent more
// than the endpoint takes. The status says so, so a forge knows the
// difference between a payload it should not send and a request the
// daemon refused.
func TestHandleForgeWebhook_OversizedBodyIsTooLarge(t *testing.T) {
	oversized := strings.Repeat("a", MaxWebhookBodyBytes+1)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhooks/github", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h := NewWebhookHandlers(nil, nil, nil)
	h.HandleForgeWebhook(rr, req, "company-github", config.ForgeGitHub)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("the oversized body was answered with %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

// The answer tells the forge what happened. A request that triggered no
// build says so, instead of saying "received".
func TestHandleForgeWebhook_AnEventThatTriggersNoBuildSaysSo(t *testing.T) {
	const forgeName = "company-github"
	const secret = "s3cret"

	body := `{"not":"a push event"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhooks/github", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", "sha256=valid-signature")

	rr := httptest.NewRecorder()
	client := forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub).WithWebhookSecret(secret)
	h := &WebhookHandlers{
		webhookConfig: map[string]*config.WebhookConfig{forgeName: {Secret: secret}},
		forgeClients:  map[string]forge.Client{forgeName: client},
	}
	h.HandleForgeWebhook(rr, req, forgeName, config.ForgeGitHub)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("the request was answered with %d, want %d", rr.Code, http.StatusAccepted)
	}
	if !strings.Contains(rr.Body.String(), `"status":"ignored"`) {
		t.Fatalf("the answer does not say that nothing was triggered: %s", rr.Body.String())
	}
}

// A payload that could not be read says so: the sender has to know
// that the event was not understood.
func TestHandleForgeWebhook_APayloadThatCouldNotBeReadSaysSo(t *testing.T) {
	const forgeName = "company-github"
	const secret = "s3cret"

	body := `{"ref":"refs/heads/main"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhooks/github", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=valid-signature")

	rr := httptest.NewRecorder()
	// The mock fails to read the payload.
	client := forge.NewEnhancedMockForgeClient(forgeName, forge.TypeGitHub).
		WithWebhookSecret(secret).
		WithNetworkTimeout(50 * time.Millisecond)
	h := &WebhookHandlers{
		webhookConfig: map[string]*config.WebhookConfig{forgeName: {Secret: secret}},
		forgeClients:  map[string]forge.Client{forgeName: client},
	}
	h.HandleForgeWebhook(rr, req, forgeName, config.ForgeGitHub)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("the request was answered with %d, want %d", rr.Code, http.StatusAccepted)
	}
	if !strings.Contains(rr.Body.String(), `"status":"error"`) {
		t.Fatalf("the answer does not say that the payload was not read: %s", rr.Body.String())
	}
}
