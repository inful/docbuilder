package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/pipeline"
	"github.com/go-chi/chi/v5"
)

func TestListDLQEmpty(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()
	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq", nil)
	w := httptest.NewRecorder()

	svc.HandleListDLQ(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp DLQListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected 0 events, got %d", resp.Total)
	}
	if len(resp.Events) != 0 {
		t.Errorf("expected empty events, got %d", len(resp.Events))
	}
}

func TestListDLQWithEvents(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()

	// Add some failed events
	event1 := pipeline.SimpleEvent{E: "build-clone-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event1,
		Error:     fmt.Errorf("clone failed"),
		Timestamp: time.Now(),
	})

	event2 := pipeline.SimpleEvent{E: "build-discover-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event2,
		Error:     fmt.Errorf("discovery failed"),
		Timestamp: time.Now(),
	})

	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq", nil)
	w := httptest.NewRecorder()

	svc.HandleListDLQ(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp DLQListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("expected 2 events, got %d", resp.Total)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events in list, got %d", len(resp.Events))
	}

	if resp.Events[0].Error != "clone failed" {
		t.Errorf("expected 'clone failed', got %s", resp.Events[0].Error)
	}
	if resp.Events[0].Status != "pending_retry" {
		t.Errorf("expected status pending_retry, got %s", resp.Events[0].Status)
	}
}

func TestGetDLQEvent(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()

	event := pipeline.SimpleEvent{E: "build-clone-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event,
		Error:     fmt.Errorf("clone failed"),
		Timestamp: time.Now(),
	})

	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq/0", nil)
	w := httptest.NewRecorder()

	// Mock chi context
	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleGetDLQEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var dto DLQEventDTO
	if err := json.NewDecoder(w.Body).Decode(&dto); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if dto.ID != 0 {
		t.Errorf("expected id 0, got %d", dto.ID)
	}
	if dto.Error != "clone failed" {
		t.Errorf("expected 'clone failed', got %s", dto.Error)
	}
}

func TestGetDLQEventNotFound(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()
	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq/0", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleGetDLQEvent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetDLQEventInvalidID(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()
	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq/invalid", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "invalid"))

	svc.HandleGetDLQEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRetryDLQEvent(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()

	event := pipeline.SimpleEvent{E: "clone-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event,
		Error:     fmt.Errorf("clone failed"),
		Timestamp: time.Now(),
	})

	svc := NewDLQService(dlq)

	req := httptest.NewRequest("POST", "/admin/dlq/0/retry", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleRetryDLQEvent(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["message"] != "event marked for retry" {
		t.Errorf("expected retry message, got %v", resp["message"])
	}
}

func TestRetryDLQEventNotFound(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()
	svc := NewDLQService(dlq)

	req := httptest.NewRequest("POST", "/admin/dlq/0/retry", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleRetryDLQEvent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteDLQEvent(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()

	event1 := pipeline.SimpleEvent{E: "clone-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event1,
		Error:     fmt.Errorf("error1"),
		Timestamp: time.Now(),
	})

	event2 := pipeline.SimpleEvent{E: "discover-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event2,
		Error:     fmt.Errorf("error2"),
		Timestamp: time.Now(),
	})

	svc := NewDLQService(dlq)

	req := httptest.NewRequest("DELETE", "/admin/dlq/0", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleDeleteDLQEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify event was removed
	remaining := dlq.GetAll()
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining event, got %d", len(remaining))
	}
}

func TestDeleteDLQEventNotFound(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()
	svc := NewDLQService(dlq)

	req := httptest.NewRequest("DELETE", "/admin/dlq/0", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleDeleteDLQEvent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDLQStats(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()

	event1 := pipeline.SimpleEvent{E: "clone-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event1,
		Error:     fmt.Errorf("error1"),
		Timestamp: time.Now(),
	})

	event2 := pipeline.SimpleEvent{E: "discover-1"}
	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event2,
		Error:     fmt.Errorf("error2"),
		Timestamp: time.Now(),
	})

	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq/stats", nil)
	w := httptest.NewRecorder()

	svc.HandleDLQStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if totalFailed, ok := stats["total_failed_events"].(float64); !ok || int(totalFailed) != 2 {
		t.Errorf("expected 2 total_failed_events, got %v", stats["total_failed_events"])
	}

	if pendingRetry, ok := stats["pending_retry"].(float64); !ok || int(pendingRetry) != 2 {
		t.Errorf("expected 2 pending_retry, got %v", stats["pending_retry"])
	}
}

func TestExtractBuildID(t *testing.T) {
	svc := NewDLQService(pipeline.NewDeadLetterQueue())

	tests := []struct {
		event    pipeline.Event
		expected string
	}{
		{
			pipeline.SimpleEvent{E: "clone-1"},
			"unknown",
		},
		{
			nil,
			"unknown",
		},
	}

	for i, tt := range tests {
		result := svc.extractBuildID(tt.event)
		if result != tt.expected {
			t.Errorf("[%d] expected %s, got %s", i, tt.expected, result)
		}
	}
}

func TestExtractEventType(t *testing.T) {
	svc := NewDLQService(pipeline.NewDeadLetterQueue())

	tests := []struct {
		event    pipeline.Event
		expected string
	}{
		{
			pipeline.SimpleEvent{E: "clone-1"},
			"SimpleEvent[clone-1]",
		},
		{
			nil,
			"unknown",
		},
	}

	for i, tt := range tests {
		result := svc.extractEventType(tt.event)
		if result != tt.expected {
			t.Errorf("[%d] expected %s, got %s", i, tt.expected, result)
		}
	}
}

func TestAdminAuthMiddleware(t *testing.T) {
	validTokens := map[string]bool{
		"token123": true,
		"token456": true,
	}

	middleware := adminAuthMiddleware(validTokens)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			"missing auth header",
			"",
			http.StatusUnauthorized,
		},
		{
			"invalid format",
			"InvalidFormat token123",
			http.StatusUnauthorized,
		},
		{
			"invalid token",
			"Bearer invalid",
			http.StatusForbidden,
		},
		{
			"valid token",
			"Bearer token123",
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/admin/dlq", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestDLQEventDTOFields(t *testing.T) {
	dlq := pipeline.NewDeadLetterQueue()
	event := pipeline.SimpleEvent{E: "clone-1"}
	ts := time.Now()

	dlq.Enqueue(pipeline.FailedEvent{
		Event:     event,
		Error:     fmt.Errorf("test error"),
		Timestamp: ts,
	})

	svc := NewDLQService(dlq)

	req := httptest.NewRequest("GET", "/admin/dlq/0", nil)
	w := httptest.NewRecorder()

	req = req.WithContext(testContextWithURLParam(req.Context(), "id", "0"))

	svc.HandleGetDLQEvent(w, req)

	var dto DLQEventDTO
	if err := json.NewDecoder(w.Body).Decode(&dto); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if dto.ID != 0 {
		t.Errorf("expected ID 0, got %d", dto.ID)
	}
	if dto.EventType != "SimpleEvent[clone-1]" {
		t.Errorf("expected EventType SimpleEvent[clone-1], got %s", dto.EventType)
	}
	if dto.Error != "test error" {
		t.Errorf("expected Error 'test error', got %s", dto.Error)
	}
	if dto.Status != "pending_retry" {
		t.Errorf("expected Status pending_retry, got %s", dto.Status)
	}
	if dto.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
}

// Helper function for testing with chi URL params
func testContextWithURLParam(ctx context.Context, key string, value string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}
