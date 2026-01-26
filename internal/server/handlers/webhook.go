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

// HandleForgeWebhook handles a webhook for a specific configured forge instance.
//
// The forgeName is the configured forge instance name (config.forges[].name),
// not the forge type (github/gitlab/forgejo).
func (h *WebhookHandlers) HandleForgeWebhook(w http.ResponseWriter, r *http.Request, forgeName string, forgeType config.ForgeType) {
	switch forgeType {
	case config.ForgeGitHub:
		h.handleForgeWebhookWithValidation(w, r, "X-GitHub-Event", "X-Hub-Signature-256", forgeName)
		return
	case config.ForgeGitLab:
		h.handleForgeWebhookWithValidation(w, r, "X-Gitlab-Event", "X-Gitlab-Token", forgeName)
		return
	case config.ForgeForgejo:
		// Forgejo uses X-Forgejo-Event or X-Gitea-Event
		eventHeader := "X-Forgejo-Event"
		if r.Header.Get(eventHeader) == "" {
			eventHeader = "X-Gitea-Event"
		}
		h.handleForgeWebhookWithValidation(w, r, eventHeader, "X-Hub-Signature-256", forgeName)
		return
	case config.ForgeLocal:
		err := errors.ValidationError("webhooks are not supported for local forge").
			WithContext("forge", forgeName).
			WithContext("type", string(forgeType)).
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	default:
		err := errors.ValidationError("unsupported forge type for webhook handler").
			WithContext("forge", forgeName).
			WithContext("type", string(forgeType)).
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
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
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	// Read raw payload for logging or future signature checks
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		derr := errors.ValidationError("invalid JSON payload").
			WithContext("content_type", r.Header.Get("Content-Type")).
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, derr)
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
		h.errorAdapter.WriteErrorResponse(w, r, derr)
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
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	// Read body for validation and parsing
	body, err := io.ReadAll(r.Body)
	if err != nil {
		derr := errors.ValidationError("failed to read request body").
			WithContext("error", err.Error()).
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, derr)
		return
	}

	// Validate webhook signature if configured
	if validationErr := h.validateWebhookSignature(forgeName, signatureHeader, body, r); validationErr != nil {
		h.errorAdapter.WriteErrorResponse(w, r, validationErr)
		return
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
	jobID := h.triggerBuildFromEvent(event, forgeName)

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
		h.errorAdapter.WriteErrorResponse(w, r, derr)
		return
	}
}

// validateWebhookSignature validates webhook signature if configured.
// Returns nil if validation passes or is not configured, error otherwise.
func (h *WebhookHandlers) validateWebhookSignature(forgeName, signatureHeader string, body []byte, r *http.Request) error {
	if h.webhookConfig == nil {
		return nil
	}

	whCfg, ok := h.webhookConfig[forgeName]
	if !ok || whCfg == nil || whCfg.Secret == "" {
		return nil
	}

	signature := r.Header.Get(signatureHeader)
	if signature == "" && signatureHeader == "X-Gitlab-Token" {
		// GitLab uses token in header directly
		signature = r.Header.Get("X-Gitlab-Token")
	}

	client, ok := h.forgeClients[forgeName]
	if !ok || client == nil {
		return nil
	}

	if !client.ValidateWebhook(body, signature, whCfg.Secret) {
		slog.Warn("Webhook signature validation failed",
			"forge", forgeName,
			"event", r.Header.Get("X-GitHub-Event"))
		return errors.ValidationError("webhook signature validation failed").
			WithContext("forge", forgeName).
			Build()
	}

	slog.Debug("Webhook signature validated", "forge", forgeName)
	return nil
}

// triggerBuildFromEvent triggers a build from a webhook event if valid.
// Returns the job ID if a build was triggered, empty string otherwise.
func (h *WebhookHandlers) triggerBuildFromEvent(event *forge.WebhookEvent, forgeName string) string {
	if event == nil || event.Repository == nil || h.trigger == nil {
		return ""
	}

	// Extract branch from event
	branch := event.Branch
	if branch == "" && len(event.Commits) > 0 {
		// Try to extract from ref (e.g., "refs/heads/main" -> "main")
		if ref, ok := event.Metadata["ref"]; ok {
			if after, ok0 := strings.CutPrefix(ref, "refs/heads/"); ok0 {
				branch = after
			}
		}
	}

	jobID := h.trigger.TriggerWebhookBuild(event.Repository.FullName, branch)
	if jobID != "" {
		slog.Info("Webhook triggered build",
			"forge", forgeName,
			"repo", event.Repository.FullName,
			"branch", branch,
			"job_id", jobID)
	}

	return jobID
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
