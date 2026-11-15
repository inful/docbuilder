package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The generic webhook endpoint reads the whole body. It must not read an
// unbounded body: the webhook port is the one port that has to be
// reachable from the internet.
func TestHandleWebhook_OversizedBodyIsRejected(t *testing.T) {
	oversized := strings.Repeat("a", MaxWebhookBodyBytes+1)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h := NewWebhookHandlers(nil, nil, nil)
	h.HandleGenericWebhook(rr, req)

	if rr.Code == http.StatusAccepted {
		t.Fatalf("expected the oversized payload to be rejected, got %d", rr.Code)
	}
}

// A normal payload still works.
func TestHandleWebhook_NormalBodyIsAccepted(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", bytes.NewBufferString(`{"hello":"world"}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h := NewWebhookHandlers(nil, nil, nil)
	h.HandleGenericWebhook(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
}

// The forge path reads the body too, and must have the same bound.
func TestHandleForgeWebhook_OversizedBodyIsRejected(t *testing.T) {
	oversized := strings.Repeat("a", MaxWebhookBodyBytes+1)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhooks/github", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h := NewWebhookHandlers(nil, nil, nil)
	h.HandleForgeWebhook(rr, req, "company-github", config.ForgeGitHub)

	if rr.Code == http.StatusAccepted {
		t.Fatalf("expected the oversized payload to be rejected, got %d", rr.Code)
	}
}
