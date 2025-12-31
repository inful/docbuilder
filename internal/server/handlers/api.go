package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
)

// APIHandlers contains API-related HTTP handlers.
type APIHandlers struct {
	config       *config.Config
	daemon       DaemonAPIInterface
	errorAdapter *errors.HTTPErrorAdapter
}

// DaemonAPIInterface defines the daemon methods needed by API handlers.
type DaemonAPIInterface interface {
	GetStatus() string
	GetStartTime() time.Time
}

// NewAPIHandlers creates a new API handlers instance.
func NewAPIHandlers(config *config.Config, daemon DaemonAPIInterface) *APIHandlers {
	return &APIHandlers{
		config:       config,
		daemon:       daemon,
		errorAdapter: errors.NewHTTPErrorAdapter(slog.Default()),
	}
}

// HandleDocsStatus handles the documentation status endpoint.
func (h *APIHandlers) HandleDocsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	status := &responses.ServerStatusResponse{
		Status:      "ready",
		Title:       h.config.Hugo.Title,
		Description: h.config.Hugo.Description,
		Theme:       "relearn",
		BaseURL:     h.config.Hugo.BaseURL,
		OutputDir:   h.config.Output.Directory,
		Timestamp:   time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to write docs status response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}

// HandleDaemonStatus handles the daemon status endpoint.
func (h *APIHandlers) HandleDaemonStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	status := &responses.DaemonStatusResponse{
		Status:    h.daemon.GetStatus(),
		Uptime:    time.Since(h.daemon.GetStartTime()).Seconds(),
		StartTime: h.daemon.GetStartTime(),
		Config: responses.DaemonConfigSummary{
			ForgesCount:      len(h.config.Forges),
			SyncSchedule:     h.config.Daemon.Sync.Schedule,
			ConcurrentBuilds: h.config.Daemon.Sync.ConcurrentBuilds,
			QueueSize:        h.config.Daemon.Sync.QueueSize,
		},
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode daemon status").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}

// HandleDaemonConfig handles the daemon configuration endpoint.
func (h *APIHandlers) HandleDaemonConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	// Return sanitized configuration (no secrets)
	configSummary := h.sanitizeConfig(h.config)
	response := &responses.ConfigResponse{
		Status:    "ok",
		Config:    configSummary,
		Timestamp: time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, response); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to write config response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}

// sanitizeConfig creates a sanitized view of the configuration without secrets.
func (h *APIHandlers) sanitizeConfig(cfg *config.Config) responses.ConfigSummary {
	// Create sanitized forge summaries
	forges := make([]responses.ForgeSummary, len(cfg.Forges))
	for i, forge := range cfg.Forges {
		forges[i] = responses.ForgeSummary{
			Name:          forge.Name,
			Type:          string(forge.Type),
			BaseURL:       forge.BaseURL,
			Organizations: forge.Organizations,
			Groups:        forge.Groups,
			AutoDiscover:  forge.AutoDiscover,
			// Note: Auth details are intentionally omitted for security
		}
	}

	return responses.ConfigSummary{
		Hugo: responses.HugoSummary{
			Title:       cfg.Hugo.Title,
			Theme:       "relearn",
			BaseURL:     cfg.Hugo.BaseURL,
			Description: cfg.Hugo.Description,
		},
		Daemon: responses.DaemonSummary{
			DocsPort:    cfg.Daemon.HTTP.DocsPort,
			WebhookPort: cfg.Daemon.HTTP.WebhookPort,
			AdminPort:   cfg.Daemon.HTTP.AdminPort,
		},
		Forges: forges,
	}
}
