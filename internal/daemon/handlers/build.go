package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/responses"
)

// BuildHandlers contains build and discovery related HTTP handlers
type BuildHandlers struct {
	daemon DaemonBuildInterface
}

// DaemonBuildInterface defines the daemon methods needed by build handlers
type DaemonBuildInterface interface {
	TriggerDiscovery() string
	TriggerBuild() string
	GetQueueLength() int
	GetActiveJobs() int
}

// NewBuildHandlers creates a new build handlers instance
func NewBuildHandlers(daemon DaemonBuildInterface) *BuildHandlers {
	return &BuildHandlers{daemon: daemon}
}

// HandleTriggerDiscovery handles the discovery trigger endpoint
func (h *BuildHandlers) HandleTriggerDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement discovery triggering
	if h.daemon != nil {
		jobID := h.daemon.TriggerDiscovery()
		response := &responses.TriggerResponse{
			Status: "triggered",
			JobID:  jobID,
		}
		if err := writeJSON(w, http.StatusOK, response); err != nil {
			slog.Error("failed to encode discovery trigger response", "error", err)
		}
	} else {
		http.Error(w, "Daemon not available", http.StatusServiceUnavailable)
	}
}

// HandleTriggerBuild handles the build trigger endpoint
func (h *BuildHandlers) HandleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement build triggering
	if h.daemon != nil {
		jobID := h.daemon.TriggerBuild()
		response := &responses.TriggerResponse{
			Status: "triggered",
			JobID:  jobID,
		}
		if err := writeJSON(w, http.StatusOK, response); err != nil {
			slog.Error("failed to encode build trigger response", "error", err)
		}
	} else {
		http.Error(w, "Daemon not available", http.StatusServiceUnavailable)
	}
}

// HandleBuildStatus handles the build status endpoint
func (h *BuildHandlers) HandleBuildStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement build status tracking
	status := &responses.BuildStatusResponse{
		Status:      "ok",
		QueueLength: h.daemon.GetQueueLength(),
		Statistics: responses.BuildStatistics{
			TotalBuilds:     0, // TODO: Get from daemon state
			SuccessfulBuilds: 0,
			FailedBuilds:    0,
		},
		Timestamp: time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		slog.Error("failed to encode build status", "error", err)
	}
}

// HandleRepositories handles the repositories endpoint
func (h *BuildHandlers) HandleRepositories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement repository listing from state
	repos := &responses.RepositoryStatusResponse{
		Status:       "ok",
		Repositories: []responses.RepositoryInfo{}, // TODO: Get from daemon state
		Timestamp:    time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, repos); err != nil {
		slog.Error("failed to encode repositories", "error", err)
	}
}