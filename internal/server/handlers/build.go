package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
	"git.home.luguber.info/inful/docbuilder/internal/errors"
)

// BuildHandlers contains build and discovery related HTTP handlers
type BuildHandlers struct {
	daemon       DaemonBuildInterface
	errorAdapter *errors.HTTPErrorAdapter
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
	return &BuildHandlers{
		daemon:       daemon,
		errorAdapter: errors.NewHTTPErrorAdapter(slog.Default()),
	}
}

// HandleTriggerDiscovery handles the discovery trigger endpoint
func (h *BuildHandlers) HandleTriggerDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
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
			internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode discovery trigger response").
				Build()
			h.errorAdapter.WriteErrorResponse(w, internalErr)
		}
	} else {
		err := errors.DaemonError("daemon not available").
			WithContext("service", "discovery").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
	}
}

// HandleTriggerBuild handles the build trigger endpoint
func (h *BuildHandlers) HandleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "POST").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
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
			internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode build trigger response").
				Build()
			h.errorAdapter.WriteErrorResponse(w, internalErr)
		}
	} else {
		err := errors.DaemonError("daemon not available").
			WithContext("service", "build").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
	}
}

// HandleBuildStatus handles the build status endpoint
func (h *BuildHandlers) HandleBuildStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	// TODO: Implement build status tracking
	status := &responses.BuildStatusResponse{
		Status:      "ok",
		QueueLength: h.daemon.GetQueueLength(),
		Statistics: responses.BuildStatistics{
			TotalBuilds:      0, // TODO: Get from daemon state
			SuccessfulBuilds: 0,
			FailedBuilds:     0,
		},
		Timestamp: time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode build status").
			Build()
		h.errorAdapter.WriteErrorResponse(w, internalErr)
	}
}

// HandleRepositories handles the repositories endpoint
func (h *BuildHandlers) HandleRepositories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.ValidationError("invalid HTTP method").
			WithContext("method", r.Method).
			WithContext("allowed_method", "GET").
			Build()
		h.errorAdapter.WriteErrorResponse(w, err)
		return
	}

	// TODO: Implement repository listing from state
	repos := &responses.RepositoryStatusResponse{
		Status:       "ok",
		Repositories: []responses.RepositoryInfo{}, // TODO: Get from daemon state
		Timestamp:    time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, repos); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to encode repositories").
			Build()
		h.errorAdapter.WriteErrorResponse(w, internalErr)
	}
}
