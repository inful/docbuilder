// Package handlers contains HTTP handlers for build and discovery operations.
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
)

// BuildHandlers contains build and discovery related HTTP handlers.
type BuildHandlers struct {
	daemon       DaemonBuildInterface
	errorAdapter *errors.HTTPErrorAdapter
}

// DaemonBuildInterface defines the daemon methods needed by build handlers.
type DaemonBuildInterface interface {
	TriggerDiscovery() string
	TriggerBuild() string
	GetQueueLength() int
	GetActiveJobs() int
}

// NewBuildHandlers creates a new build handlers instance.
func NewBuildHandlers(daemon DaemonBuildInterface) *BuildHandlers {
	return &BuildHandlers{
		daemon:       daemon,
		errorAdapter: errors.NewHTTPErrorAdapter(slog.Default()),
	}
}

// handleTriggerAction is a generic handler for trigger endpoints.
func (h *BuildHandlers) handleTriggerAction(w http.ResponseWriter, r *http.Request, serviceName string, triggerFunc func() string, errMsg string) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	if h.daemon != nil {
		jobID := triggerFunc()
		response := &responses.TriggerResponse{
			Status: "triggered",
			JobID:  jobID,
		}
		if err := writeJSON(w, http.StatusOK, response); err != nil {
			internalErr := errors.WrapError(err, errors.CategoryInternal, errMsg).
				Build()
			h.errorAdapter.WriteErrorResponse(w, r, internalErr)
		}
	} else {
		err := errors.DaemonError("daemon not available").
			WithContext("service", serviceName).
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
	}
}

// HandleTriggerDiscovery handles the discovery trigger endpoint.
func (h *BuildHandlers) HandleTriggerDiscovery(w http.ResponseWriter, r *http.Request) {
	h.handleTriggerAction(w, r, "discovery", h.daemon.TriggerDiscovery, "failed to encode discovery trigger response")
}

// HandleTriggerBuild handles the build trigger endpoint.
func (h *BuildHandlers) HandleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	h.handleTriggerAction(w, r, "build", h.daemon.TriggerBuild, "failed to encode build trigger response")
}

// HandleBuildStatus handles the build status endpoint.
func (h *BuildHandlers) HandleBuildStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	status := &responses.BuildStatusResponse{
		Status:      "ok",
		QueueLength: h.daemon.GetQueueLength(),
		Statistics: responses.BuildStatistics{
			TotalBuilds:      0, // populated by daemon state when available
			SuccessfulBuilds: 0,
			FailedBuilds:     0,
		},
		Timestamp: time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode build status").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}

// HandleRepositories handles the repositories endpoint.
func (h *BuildHandlers) HandleRepositories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	repos := &responses.RepositoryStatusResponse{
		Status:       "ok",
		Repositories: []responses.RepositoryInfo{}, // populated from daemon state when available
		Timestamp:    time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, repos); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode repositories").
			Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
	}
}
