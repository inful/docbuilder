package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/responses"
)

// MonitoringHandlers contains monitoring-related HTTP handlers
type MonitoringHandlers struct {
	daemon DaemonInterface
}

// DaemonInterface defines the daemon methods needed by monitoring handlers  
type DaemonInterface interface {
	GetStatus() interface{} // DaemonStatus implements String() method
	GetActiveJobs() int
	GetStartTime() time.Time
}

// NewMonitoringHandlers creates a new monitoring handlers instance
func NewMonitoringHandlers(daemon DaemonInterface) *MonitoringHandlers {
	return &MonitoringHandlers{daemon: daemon}
}

// HandleHealthCheck handles the health check endpoint
func (h *MonitoringHandlers) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := &responses.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "2.0", // TODO: Get from build info
		Uptime:    time.Since(h.daemon.GetStartTime()).Seconds(),
	}

	// Check daemon health
	if h.daemon != nil {
		if status, ok := h.daemon.GetStatus().(fmt.Stringer); ok {
			health.DaemonStatus = status.String()
		}
		health.ActiveJobs = h.daemon.GetActiveJobs()
	}

	if err := writeJSONPretty(w, r, http.StatusOK, health); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
}

// HandleMetrics handles the metrics endpoint (placeholder)
func (h *MonitoringHandlers) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement Prometheus-style metrics
	metrics := &responses.MetricsResponse{
		Status:                   "ok",
		Timestamp:               time.Now().UTC(),
		HTTPRequestsTotal:       0, // TODO: Implement counters
		ActiveJobs:              h.daemon.GetActiveJobs(),
		LastDiscoveryDuration:   0, // TODO: Track discovery timing
		LastBuildDuration:       0, // TODO: Track build timing
		RepositoriesTotal:       0, // TODO: Count managed repositories
	}

	if err := writeJSONPretty(w, r, http.StatusOK, metrics); err != nil {
		slog.Error("Failed to write metrics response", "error", err)
	}
}