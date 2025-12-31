package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// MonitoringHandlers contains monitoring-related HTTP handlers.
type MonitoringHandlers struct {
	daemon       DaemonInterface
	errorAdapter *errors.HTTPErrorAdapter
}

// DaemonInterface defines the daemon methods needed by monitoring handlers.
type DaemonInterface interface {
	GetStatus() string
	GetActiveJobs() int
	GetStartTime() time.Time
	// Live metrics (optional; return zero values when unavailable)
	HTTPRequestsTotal() int
	RepositoriesTotal() int
	LastDiscoveryDurationSec() int
	LastBuildDurationSec() int
}

// NewMonitoringHandlers creates a new monitoring handlers instance.
func NewMonitoringHandlers(daemon DaemonInterface) *MonitoringHandlers {
	return &MonitoringHandlers{
		daemon:       daemon,
		errorAdapter: errors.NewHTTPErrorAdapter(slog.Default()),
	}
}

// HandleHealthCheck handles the health check endpoint.
func (h *MonitoringHandlers) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	health := &responses.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   version.Version,
		Uptime:    time.Since(h.daemon.GetStartTime()).Seconds(),
	}

	// Check daemon health
	if h.daemon != nil {
		health.DaemonStatus = h.daemon.GetStatus()
		health.ActiveJobs = h.daemon.GetActiveJobs()
	}

	if err := writeJSONPretty(w, r, http.StatusOK, health); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to write health response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}

// HandleMetrics handles the metrics endpoint (placeholder).
func (h *MonitoringHandlers) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	metrics := &responses.MetricsResponse{
		Status:                "ok",
		Timestamp:             time.Now().UTC(),
		HTTPRequestsTotal:     h.daemon.HTTPRequestsTotal(),
		ActiveJobs:            h.daemon.GetActiveJobs(),
		LastDiscoveryDuration: h.daemon.LastDiscoveryDurationSec(),
		LastBuildDuration:     h.daemon.LastBuildDurationSec(),
		RepositoriesTotal:     h.daemon.RepositoriesTotal(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, metrics); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to write metrics response").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}
