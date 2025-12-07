package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the API server.
type Server struct {
	Addr            string
	router          *chi.Mux
	server          *http.Server
	eventSubscriber *EventSubscriber
}

// NewServer creates a new API server.
func NewServer(addr string) *Server {
	s := &Server{
		Addr:            addr,
		router:          chi.NewRouter(),
		eventSubscriber: NewEventSubscriber(),
	}

	s.setupRoutes()

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	// Health check
	s.router.Get("/health", s.handleHealth)

	// Build routes
	s.router.Post("/builds", s.handleCreateBuild)
	s.router.Get("/builds", s.handleListBuilds)
	s.router.Get("/builds/{id}", s.handleGetBuild)
	s.router.Put("/builds/{id}/status", s.handleUpdateBuildStatus)

	// Build events (Server-Sent Events)
	s.router.Get("/builds/{id}/events", s.HandleBuildEvents(s.eventSubscriber))

	// Metrics endpoint
	s.router.Get("/metrics", s.handleMetrics)
}

// Start starts the API server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Response represents a standard API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Error writes an error response.
func (s *Server) Error(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := Response{
		Success: false,
		Error:   message,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Success writes a success response.
func (s *Server) Success(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := Response{
		Success: true,
		Data:    data,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Handler methods

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := observability.GetMetricsCollector().GetSnapshot()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(metrics.FormatMetrics()))
}
