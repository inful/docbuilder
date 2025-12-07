package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIServerCreation(t *testing.T) {
	srv := NewServer(":8080")
	if srv == nil {
		t.Fatal("expected server, got nil")
	}
	if srv.Addr != ":8080" {
		t.Errorf("expected addr :8080, got %s", srv.Addr)
	}
	if srv.server == nil {
		t.Fatal("expected http.Server, got nil")
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer(":8080")
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()
	if body != `{"status":"healthy"}` {
		t.Errorf("expected healthy response, got %s", body)
	}
}

func TestCreateBuildEndpoint(t *testing.T) {
	srv := NewServer(":8080")
	req := BuildRequest{
		Name: "test-build",
	}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/builds", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}

	if resp.Data == nil {
		t.Error("expected data in response")
	}
}

func TestListBuildsEndpoint(t *testing.T) {
	srv := NewServer(":8080")
	req := httptest.NewRequest("GET", "/builds", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
}

func TestGetBuildEndpoint(t *testing.T) {
	srv := NewServer(":8080")
	req := httptest.NewRequest("GET", "/builds/build-123", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}

	data := resp.Data.(map[string]interface{})
	if data["id"] != "build-123" {
		t.Errorf("expected id build-123, got %v", data["id"])
	}
}

func TestUpdateBuildStatusEndpoint(t *testing.T) {
	srv := NewServer(":8080")
	updateReq := map[string]string{"status": "completed"}
	body, _ := json.Marshal(updateReq)
	httpReq := httptest.NewRequest("PUT", "/builds/build-123/status", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
}

func TestBuildEventsEndpoint(t *testing.T) {
	srv := NewServer(":8080")
	req := httptest.NewRequest("GET", "/builds/build-123/events", nil)
	w := httptest.NewRecorder()

	// Create a channel to wait for the handler to complete
	done := make(chan bool, 1)
	go func() {
		srv.router.ServeHTTP(w, req)
		done <- true
	}()

	// Wait for handler to send initial event or timeout
	select {
	case <-time.After(2 * time.Second):
		// Expected timeout after SSE handler completes
	case <-done:
		// Handler completed
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check SSE headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cc)
	}
}

func TestServerErrorResponse(t *testing.T) {
	srv := NewServer(":8080")
	w := httptest.NewRecorder()

	srv.Error(w, nil, http.StatusNotFound, "build not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success=false, got %v", resp.Success)
	}

	if resp.Error != "build not found" {
		t.Errorf("expected error message 'build not found', got %s", resp.Error)
	}
}

func TestServerSuccessResponse(t *testing.T) {
	srv := NewServer(":8080")
	w := httptest.NewRecorder()

	testData := map[string]string{"id": "123", "name": "test"}
	srv.Success(w, http.StatusOK, testData)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}

	if resp.Error != "" {
		t.Errorf("expected no error, got %s", resp.Error)
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	srv := NewServer(":9999") // Use high port to avoid conflicts

	// Start server
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown gracefully
	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone <- srv.Shutdown(ctx)
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Errorf("shutdown failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
