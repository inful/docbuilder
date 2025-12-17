// Package handlers provides HTTP handlers for webhook endpoints across different forge providers.
package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// WebhookTrigger provides the interface for triggering webhook-based builds.
type WebhookTrigger interface {
	TriggerWebhookBuild(repoFullName, branch string) string
}

// WebhookHandlers contains HTTP handlers for webhook integrations.
type WebhookHandlers struct {
	errorAdapter  *errors.HTTPErrorAdapter
	trigger       WebhookTrigger
	forgeClients  map[string]forge.Client
	webhookConfig map[string]*config.WebhookConfig
}

// NewWebhookHandlers constructs a new WebhookHandlers.
func NewWebhookHandlers(trigger WebhookTrigger, forgeClients map[string]forge.Client, webhookConfig map[string]*config.WebhookConfig) *WebhookHandlers {
	return &WebhookHandlers{
		errorAdapter:  errors.NewHTTPErrorAdapter(slog.Default()),
		trigger:       trigger,
		forgeClients:  forgeClients,
		webhookConfig: webhookConfig,
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

// handleForgeWebhookWithValidation validates webhook signature and triggers builds.
func (h *WebhookHandlers) handleForgeWebhookWithValidation(w http.ResponseWriter, r *http.Request, eventHeader, signatureHeader, forgeName string) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	// Read body for validation and parsing
	body, err := io.ReadAll(r.Body)
	if err != nil {
		derr := errors.ValidationError("failed to read request body").
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}

	// Validate webhook signature if configured
	if h.webhookConfig != nil {
		if whCfg, ok := h.webhookConfig[forgeName]; ok && whCfg != nil && whCfg.Secret != "" {
			signature := r.Header.Get(signatureHeader)
			if signature == "" && signatureHeader == "X-Gitlab-Token" {
				// GitLab uses token in header directly
				signature = r.Header.Get("X-Gitlab-Token")
			}

			if client, ok := h.forgeClients[forgeName]; ok && client != nil {
				if !client.ValidateWebhook(body, signature, whCfg.Secret) {
					slog.Warn("Webhook signature validation failed",
						"forge", forgeName,
						"event", r.Header.Get(eventHeader))
					err := errors.ValidationError("webhook signature validation failed").
						WithContext("forge", forgeName).
						Build()
					h.errorAdapter.WriteErrorResponse(w, err)
					return
				}
				slog.Debug("Webhook signature validated", "forge", forgeName)
			}
		}
	}

	// Parse webhook event
	eventType := r.Header.Get(eventHeader)
	var event *forge.WebhookEvent
	if client, ok := h.forgeClients[forgeName]; ok && client != nil {
		event, err = client.ParseWebhookEvent(body, eventType)
		if err != nil {
			slog.Warn("Failed to parse webhook event",
				"forge", forgeName,
				"event", eventType,
				"error", err)
			// Continue even if parsing fails - acknowledge receipt
		}
	}

	// Trigger build for the repository if event was parsed successfully
	var jobID string
	if event != nil && event.Repository != nil && h.trigger != nil {
		// Extract branch from event
		branch := event.Branch
		if branch == "" && len(event.Commits) > 0 {
			// Try to extract from ref (e.g., "refs/heads/main" -> "main")
			if ref, ok := event.Metadata["ref"]; ok {
				if strings.HasPrefix(ref, "refs/heads/") {
					branch = strings.TrimPrefix(ref, "refs/heads/")
				}
			}
		}

		jobID = h.trigger.TriggerWebhookBuild(event.Repository.FullName, branch)
		if jobID != "" {
			slog.Info("Webhook triggered build",
				"forge", forgeName,
				"repo", event.Repository.FullName,
				"branch", branch,
				"job_id", jobID)
		}
	}

	resp := map[string]any{
		"status":    "received",
		"timestamp": time.Now().UTC(),
		"event":     eventType,
		"source":    forgeName,
	}
	if jobID != "" {
		resp["build_job_id"] = jobID
	}

	if err := writeJSONPretty(w, r, http.StatusAccepted, resp); err != nil {
		derr := errors.WrapError(err, errors.CategoryInternal, "failed to write webhook response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, derr)
		return
	}
}

// HandleGitHubWebhook handles GitHub webhooks.
func (h *WebhookHandlers) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleForgeWebhookWithValidation(w, r, "X-GitHub-Event", "X-Hub-Signature-256", "github")
}

// HandleGitLabWebhook handles GitLab webhooks.
func (h *WebhookHandlers) HandleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleForgeWebhookWithValidation(w, r, "X-Gitlab-Event", "X-Gitlab-Token", "gitlab")
}

// HandleForgejoWebhook handles Forgejo (Gitea-compatible) webhooks.
func (h *WebhookHandlers) HandleForgejoWebhook(w http.ResponseWriter, r *http.Request) {
	// Forgejo uses X-Forgejo-Event or X-Gitea-Event
	eventHeader := "X-Forgejo-Event"
	if r.Header.Get(eventHeader) == "" {
		eventHeader = "X-Gitea-Event"
	}
	h.handleForgeWebhookWithValidation(w, r, eventHeader, "X-Hub-Signature-256", "forgejo")
}
