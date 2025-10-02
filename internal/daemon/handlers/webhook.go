package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// WebhookHandlers contains webhook-related HTTP handlers
type WebhookHandlers struct {
	// Add any dependencies needed for webhook processing
}

// NewWebhookHandlers creates a new webhook handlers instance
func NewWebhookHandlers() *WebhookHandlers {
	return &WebhookHandlers{}
}

// HandleGitHubWebhook handles GitHub webhook requests
func (h *WebhookHandlers) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleWebhookRequest(w, r, string(config.ForgeGitHub))
}

// HandleGitLabWebhook handles GitLab webhook requests
func (h *WebhookHandlers) HandleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleWebhookRequest(w, r, string(config.ForgeGitLab))
}

// HandleForgejoWebhook handles Forgejo webhook requests
func (h *WebhookHandlers) HandleForgejoWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleWebhookRequest(w, r, string(config.ForgeForgejo))
}

// HandleGenericWebhook handles generic webhook requests with auto-detection
func (h *WebhookHandlers) HandleGenericWebhook(w http.ResponseWriter, r *http.Request) {
	// Auto-detect forge type from headers
	forgeType := h.detectForgeType(r)
	h.handleWebhookRequest(w, r, forgeType)
}

// handleWebhookRequest processes webhook requests for any forge type
func (h *WebhookHandlers) handleWebhookRequest(w http.ResponseWriter, r *http.Request, forgeType string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement webhook processing with daemon
	slog.Info("Webhook received",
		logfields.ForgeType(forgeType),
		logfields.ContentLength(r.ContentLength),
		logfields.UserAgent(r.UserAgent()))

	// For now, just acknowledge receipt
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil { //nolint:errcheck
		slog.Error("Failed to write OK response", "error", err)
	}
}

// detectForgeType detects the forge type from request headers and user agent
func (h *WebhookHandlers) detectForgeType(r *http.Request) string {
	// Detect based on headers and user agent
	userAgent := strings.ToLower(r.UserAgent())

	if strings.Contains(userAgent, "github") || r.Header.Get("X-GitHub-Event") != "" {
		return string(config.ForgeGitHub)
	}
	if strings.Contains(userAgent, "gitlab") || r.Header.Get("X-Gitlab-Event") != "" {
		return string(config.ForgeGitLab)
	}
	if strings.Contains(userAgent, "forgejo") || strings.Contains(userAgent, "gitea") {
		return string(config.ForgeForgejo)
	}

	return "unknown"
}