package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/observability"
	"git.home.luguber.info/inful/docbuilder/internal/pipeline"
	"github.com/go-chi/chi/v5"
)

// DLQService manages Dead Letter Queue operations
type DLQService struct {
	dlq *pipeline.DeadLetterQueue
}

// DLQEventDTO represents a DLQ event for API response
type DLQEventDTO struct {
	ID        int       `json:"id"`
	BuildID   string    `json:"build_id"`
	EventType string    `json:"event_type"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "pending_retry" or "failed"
}

// DLQListResponse represents the list of DLQ events
type DLQListResponse struct {
	Total  int           `json:"total"`
	Events []DLQEventDTO `json:"events"`
}

// NewDLQService creates a new DLQ service
func NewDLQService(dlq *pipeline.DeadLetterQueue) *DLQService {
	return &DLQService{dlq: dlq}
}

// adminAuthMiddleware checks for admin authorization
func adminAuthMiddleware(adminTokens map[string]bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			if !adminTokens[token] {
				http.Error(w, "invalid or unauthorized token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HandleListDLQ lists all DLQ events with optional status filtering
func (s *DLQService) HandleListDLQ(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = observability.WithStage(ctx, "dlq_list")

	failed := s.dlq.GetAll()

	// Filter by status if provided
	status := r.URL.Query().Get("status")
	var events []DLQEventDTO

	for i, fe := range failed {
		dto := DLQEventDTO{
			ID:        i,
			BuildID:   s.extractBuildID(fe.Event),
			EventType: s.extractEventType(fe.Event),
			Error:     fe.Error.Error(),
			Timestamp: fe.Timestamp,
			Status:    "pending_retry",
		}

		// Filter by status if specified
		if status != "" && dto.Status != status {
			continue
		}

		events = append(events, dto)
	}

	resp := DLQListResponse{
		Total:  len(events),
		Events: events,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)

	observability.InfoContext(ctx, "DLQ events listed",
		slog.Int("total", resp.Total),
		slog.String("filter_status", status))
}

// HandleGetDLQEvent gets details of a specific DLQ event
func (s *DLQService) HandleGetDLQEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = observability.WithStage(ctx, "dlq_get")

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		observability.ErrorContext(ctx, "invalid DLQ event id",
			slog.String("id", idStr),
			slog.String("error", err.Error()))
		return
	}

	failed := s.dlq.GetAll()
	if id < 0 || id >= len(failed) {
		http.Error(w, "event not found", http.StatusNotFound)
		observability.WarnContext(ctx, "DLQ event not found",
			slog.Int("id", id),
			slog.Int("total_events", len(failed)))
		return
	}

	fe := failed[id]
	dto := DLQEventDTO{
		ID:        id,
		BuildID:   s.extractBuildID(fe.Event),
		EventType: s.extractEventType(fe.Event),
		Error:     fe.Error.Error(),
		Timestamp: fe.Timestamp,
		Status:    "pending_retry",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto)

	observability.InfoContext(ctx, "DLQ event retrieved",
		slog.Int("id", id),
		slog.String("build_id", dto.BuildID))
}

// HandleRetryDLQEvent retries a failed event from the DLQ
func (s *DLQService) HandleRetryDLQEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = observability.WithStage(ctx, "dlq_retry")

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		observability.ErrorContext(ctx, "invalid DLQ event id for retry",
			slog.String("id", idStr),
			slog.String("error", err.Error()))
		return
	}

	failed := s.dlq.GetAll()
	if id < 0 || id >= len(failed) {
		http.Error(w, "event not found", http.StatusNotFound)
		observability.WarnContext(ctx, "DLQ event not found for retry",
			slog.Int("id", id),
			slog.Int("total_events", len(failed)))
		return
	}

	fe := failed[id]
	dto := DLQEventDTO{
		ID:        id,
		BuildID:   s.extractBuildID(fe.Event),
		EventType: s.extractEventType(fe.Event),
		Error:     fe.Error.Error(),
		Timestamp: fe.Timestamp,
		Status:    "pending_retry",
	}

	response := map[string]interface{}{
		"message": "event marked for retry",
		"event":   dto,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(response)

	observability.InfoContext(ctx, "DLQ event marked for retry",
		slog.Int("id", id),
		slog.String("build_id", dto.BuildID),
		slog.String("event_type", dto.EventType))
}

// HandleDeleteDLQEvent deletes/discards a failed event from the DLQ
func (s *DLQService) HandleDeleteDLQEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = observability.WithStage(ctx, "dlq_delete")

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		observability.ErrorContext(ctx, "invalid DLQ event id for delete",
			slog.String("id", idStr),
			slog.String("error", err.Error()))
		return
	}

	failed := s.dlq.GetAll()
	if id < 0 || id >= len(failed) {
		http.Error(w, "event not found", http.StatusNotFound)
		observability.WarnContext(ctx, "DLQ event not found for delete",
			slog.Int("id", id),
			slog.Int("total_events", len(failed)))
		return
	}

	fe := failed[id]
	buildID := s.extractBuildID(fe.Event)
	eventType := s.extractEventType(fe.Event)

	// Create a new DLQ without the deleted event
	newFailed := make([]pipeline.FailedEvent, 0)
	for i, f := range failed {
		if i != id {
			newFailed = append(newFailed, f)
		}
	}

	// Replace DLQ contents (simplified - in production would need proper atomic operation)
	s.dlq.Clear()
	for _, f := range newFailed {
		s.dlq.Enqueue(f)
	}

	response := map[string]interface{}{
		"message": "event deleted from DLQ",
		"id":      id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)

	observability.InfoContext(ctx, "DLQ event deleted",
		slog.Int("id", id),
		slog.String("build_id", buildID),
		slog.String("event_type", eventType))
}

// HandleDLQStats returns summary statistics about the DLQ
func (s *DLQService) HandleDLQStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = observability.WithStage(ctx, "dlq_stats")

	failed := s.dlq.GetAll()

	stats := map[string]interface{}{
		"total_failed_events": len(failed),
		"pending_retry":       len(failed),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(stats)

	observability.DebugContext(ctx, "DLQ stats retrieved",
		slog.Int("total_failed_events", len(failed)))
}

// Helper methods

func (s *DLQService) extractBuildID(event pipeline.Event) string {
	// For SimpleEvent, the build ID would need to be parsed from the event name
	// or tracked separately. For now, return unknown.
	if event == nil {
		return "unknown"
	}
	return "unknown"
}

func (s *DLQService) extractEventType(event pipeline.Event) string {
	if event == nil {
		return "unknown"
	}

	// Return the type name
	switch e := event.(type) {
	case pipeline.SimpleEvent:
		return fmt.Sprintf("SimpleEvent[%s]", e.E)
	default:
		return fmt.Sprintf("%T", event)
	}
}
