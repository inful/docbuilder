// Package handlers provides HTTP handlers for webhook endpoints across different forge providers.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/errors"
)

// WebhookHandlers contains HTTP handlers for webhook integrations.
type WebhookHandlers struct {
	errorAdapter *errors.HTTPErrorAdapter
}

// NewWebhookHandlers constructs a new WebhookHandlers.
func NewWebhookHandlers() *WebhookHandlers {
	return &WebhookHandlers{
		errorAdapter: errors.NewHTTPErrorAdapter(slog.Default()),
	}
}

// HandleWebhook receives generic webhook payloads (e.g., GitHub/GitLab)
// and returns a simple acknowledgement. Signature/secret validation can
// be added in middleware or here in future passes.
func (h *WebhookHandlers) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	// Read raw payload for logging or future signature checks
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		derr := errors.ValidationError("invalid JSON payload").
			WithContext("content_type", r.Header.Get("Content-Type")).
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}

	// Minimal acknowledgment response
	resp := map[string]any{
		"status":    "received",
		"timestamp": time.Now().UTC(),
		"event":     r.Header.Get("X-GitHub-Event"),
		"source":    r.Header.Get("User-Agent"),
	}

	if err := writeJSONPretty(w, r, http.StatusAccepted, resp); err != nil {
		derr := errors.WrapError(err, errors.CategoryInternal, "failed to write webhook response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}
}

// HandleGenericWebhook handles a generic webhook endpoint.
func (h *WebhookHandlers) HandleGenericWebhook(w http.ResponseWriter, r *http.Request) {
	h.HandleWebhook(w, r)
}

// HandleGitHubWebhook handles GitHub webhooks.
func (h *WebhookHandlers) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		derr := errors.ValidationError("invalid JSON payload").
			WithContext("content_type", r.Header.Get("Content-Type")).
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}

	resp := map[string]any{
		"status":    "received",
		"timestamp": time.Now().UTC(),
		"event":     r.Header.Get("X-GitHub-Event"),
		"source":    "github",
	}
	if err := writeJSONPretty(w, r, http.StatusAccepted, resp); err != nil {
		derr := errors.WrapError(err, errors.CategoryInternal, "failed to write webhook response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}
}

// HandleGitLabWebhook handles GitLab webhooks.
func (h *WebhookHandlers) HandleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		derr := errors.ValidationError("invalid JSON payload").
			WithContext("content_type", r.Header.Get("Content-Type")).
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}

	resp := map[string]any{
		"status":    "received",
		"timestamp": time.Now().UTC(),
		"event":     r.Header.Get("X-Gitlab-Event"),
		"source":    "gitlab",
	}
	if err := writeJSONPretty(w, r, http.StatusAccepted, resp); err != nil {
		derr := errors.WrapError(err, errors.CategoryInternal, "failed to write webhook response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}
}

// HandleForgejoWebhook handles Forgejo (Gitea-compatible) webhooks.
func (h *WebhookHandlers) HandleForgejoWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		derr := errors.ValidationError("invalid JSON payload").
			WithContext("content_type", r.Header.Get("Content-Type")).
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}

	// Forgejo uses X-Gitea-Event or X-Forgejo-Event; capture either
	event := r.Header.Get("X-Forgejo-Event")
	if event == "" {
		event = r.Header.Get("X-Gitea-Event")
	}
	resp := map[string]any{
		"status":    "received",
		"timestamp": time.Now().UTC(),
		"event":     event,
		"source":    "forgejo",
	}
	if err := writeJSONPretty(w, r, http.StatusAccepted, resp); err != nil {
		derr := errors.WrapError(err, errors.CategoryInternal, "failed to write webhook response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}
}
