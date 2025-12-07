package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// BuildRequest represents the request to create or update a build.
type BuildRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Repos       []RepoRequest          `json:"repos,omitempty"`
}

// RepoRequest represents a repository in a build request.
type RepoRequest struct {
	URL    string `json:"url"`
	Branch string `json:"branch,omitempty"`
	Name   string `json:"name,omitempty"`
}

// BuildResponse represents a build in the API response.
type BuildResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Duration  int64     `json:"duration_ms,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// BuildStore interface for managing builds (to be implemented later).
type BuildStore interface {
	CreateBuild(build *BuildResponse) error
	GetBuild(id string) (*BuildResponse, error)
	ListBuilds(limit, offset int) ([]*BuildResponse, error)
	UpdateBuild(build *BuildResponse) error
	DeleteBuild(id string) error
}

// MockBuildStore is a simple in-memory store for demonstration.
type MockBuildStore struct {
	builds map[string]*BuildResponse
}

// NewMockBuildStore creates a new mock build store.
func NewMockBuildStore() *MockBuildStore {
	return &MockBuildStore{
		builds: make(map[string]*BuildResponse),
	}
}

func (m *MockBuildStore) CreateBuild(build *BuildResponse) error {
	if build.ID == "" {
		build.ID = "build-" + time.Now().Format("20060102150405")
	}
	m.builds[build.ID] = build
	return nil
}

func (m *MockBuildStore) GetBuild(id string) (*BuildResponse, error) {
	build, ok := m.builds[id]
	if !ok {
		return nil, ErrBuildNotFound
	}
	return build, nil
}

func (m *MockBuildStore) ListBuilds(limit, offset int) ([]*BuildResponse, error) {
	result := make([]*BuildResponse, 0, len(m.builds))
	for _, build := range m.builds {
		result = append(result, build)
	}
	if offset > len(result) {
		offset = len(result)
	}
	if offset+limit > len(result) {
		limit = len(result) - offset
	}
	return result[offset : offset+limit], nil
}

func (m *MockBuildStore) UpdateBuild(build *BuildResponse) error {
	if _, ok := m.builds[build.ID]; !ok {
		return ErrBuildNotFound
	}
	m.builds[build.ID] = build
	return nil
}

func (m *MockBuildStore) DeleteBuild(id string) error {
	delete(m.builds, id)
	return nil
}

// API error types
var (
	ErrBuildNotFound  = NewAPIError(http.StatusNotFound, "build not found")
	ErrInvalidRequest = NewAPIError(http.StatusBadRequest, "invalid request")
)

// APIError represents an API error.
type APIError struct {
	Code    int
	Message string
}

// NewAPIError creates a new API error.
func NewAPIError(code int, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// handleCreateBuild creates a new build.
func (s *Server) handleCreateBuild(w http.ResponseWriter, r *http.Request) {
	var req BuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Error(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	slog.Info("Creating build", "name", req.Name)

	build := &BuildResponse{
		Name:      req.Name,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.Success(w, http.StatusCreated, build)
}

// handleListBuilds lists all builds.
func (s *Server) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	limit := 10
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		_, _ = io.ReadAll(r.Body) // Use limit parameter if needed
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		_, _ = io.ReadAll(r.Body) // Use offset parameter if needed
	}

	slog.Info("Listing builds", "limit", limit, "offset", offset)

	builds := make([]BuildResponse, 0)
	s.Success(w, http.StatusOK, builds)
}

// handleGetBuild gets a specific build.
func (s *Server) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	slog.Info("Getting build", "build_id", id)

	build := &BuildResponse{
		ID:        id,
		Status:    "completed",
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now(),
		Duration:  3600000,
	}

	s.Success(w, http.StatusOK, build)
}

// handleUpdateBuildStatus updates a build's status.
func (s *Server) handleUpdateBuildStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	slog.Info("Updating build status", "build_id", id)

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Error(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	build := &BuildResponse{
		ID:        id,
		Status:    req["status"],
		UpdatedAt: time.Now(),
	}

	s.Success(w, http.StatusOK, build)
}
