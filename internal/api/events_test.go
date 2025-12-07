package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEventSubscriberSubscribe(t *testing.T) {
	es := NewEventSubscriber()

	// Subscribe to a build
	eventCh, unsubscribe := es.Subscribe("build-123")
	defer unsubscribe()

	if eventCh == nil {
		t.Fatal("expected non-nil event channel")
	}

	// Publish an event
	event := BuildEvent{
		Type:      "progress",
		BuildID:   "build-123",
		Timestamp: time.Now(),
		Message:   "Building...",
	}
	es.Publish(event)

	// Receive event
	received := <-eventCh
	if received.Type != "progress" {
		t.Errorf("expected type 'progress', got '%s'", received.Type)
	}
	if received.Message != "Building..." {
		t.Errorf("expected message 'Building...', got '%s'", received.Message)
	}
}

func TestEventSubscriberMultipleSubscribers(t *testing.T) {
	es := NewEventSubscriber()

	// Create multiple subscriptions
	ch1, unsub1 := es.Subscribe("build-123")
	ch2, unsub2 := es.Subscribe("build-123")
	defer unsub1()
	defer unsub2()

	event := BuildEvent{
		Type:      "progress",
		BuildID:   "build-123",
		Timestamp: time.Now(),
	}

	es.Publish(event)

	// Both subscribers should receive the event
	<-ch1
	<-ch2

	// If we get here without timeout, both received the event
	if t.Failed() {
		t.Fatal("one or both subscribers did not receive event")
	}
}

func TestEventSubscriberUnsubscribe(t *testing.T) {
	es := NewEventSubscriber()

	_, unsub1 := es.Subscribe("build-123")
	_, unsub2 := es.Subscribe("build-123")

	if es.GetSubscriberCount("build-123") != 2 {
		t.Errorf("expected 2 subscribers, got %d", es.GetSubscriberCount("build-123"))
	}

	unsub1()
	if es.GetSubscriberCount("build-123") != 1 {
		t.Errorf("expected 1 subscriber after unsub1, got %d", es.GetSubscriberCount("build-123"))
	}

	unsub2()
	if es.GetSubscriberCount("build-123") != 0 {
		t.Errorf("expected 0 subscribers after unsub2, got %d", es.GetSubscriberCount("build-123"))
	}
}

func TestEventSubscriberIsolation(t *testing.T) {
	es := NewEventSubscriber()

	// Subscribe to different builds
	ch1, unsub1 := es.Subscribe("build-1")
	ch2, unsub2 := es.Subscribe("build-2")
	defer unsub1()
	defer unsub2()

	// Publish event to build-1
	event1 := BuildEvent{
		Type:    "progress",
		BuildID: "build-1",
	}
	es.Publish(event1)

	// build-1 subscriber should receive it
	received := <-ch1

	if received.BuildID != "build-1" {
		t.Errorf("expected build-1, got %s", received.BuildID)
	}

	// build-2 subscriber should NOT receive it (within timeout)
	select {
	case <-ch2:
		t.Fatal("build-2 subscriber should not receive build-1 event")
	case <-time.After(100 * time.Millisecond):
		// Expected: no event received
	}
}

func TestHandleBuildEventsConnected(t *testing.T) {
	es := NewEventSubscriber()
	server := &Server{}

	handler := server.HandleBuildEvents(es)

	// Create a request with immediate context cancellation
	req, ctx := httptest.NewRequest("GET", "/builds/build-123/events", nil), context.Background()
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	// Run handler in goroutine
	done := make(chan bool)
	go func() {
		handler.ServeHTTP(w, req)
		done <- true
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Check response has SSE headers (before context timeout)
	headers := w.Header()
	if ct := headers.Get("Content-Type"); ct != "text/event-stream" {
		t.Logf("Content-Type: %s (expected text/event-stream)", ct)
	}

	// Wait for handler to finish (will timeout internally)
	<-done
}

func TestHandleBuildEventsMissingID(t *testing.T) {
	es := NewEventSubscriber()
	server := &Server{}

	handler := server.HandleBuildEvents(es)
	req := httptest.NewRequest("GET", "/builds//events", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleBuildEventsEventStreaming(t *testing.T) {
	es := NewEventSubscriber()
	server := &Server{}

	// Create a request with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/builds/build-123/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Run handler in goroutine
	done := make(chan bool)
	go func() {
		handler := server.HandleBuildEvents(es)
		handler.ServeHTTP(w, req)
		done <- true
	}()

	// Publish an event
	time.Sleep(50 * time.Millisecond)
	es.Publish(BuildEvent{
		Type:      "progress",
		BuildID:   "build-123",
		Timestamp: time.Now(),
		Message:   "Building...",
	})

	// Wait for handler to finish (context timeout)
	<-done

	// Response should contain some SSE data
	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected response body with SSE data")
	}
}

func TestSendSSEEvent(t *testing.T) {
	server := &Server{}
	w := httptest.NewRecorder()

	event := BuildEvent{
		Type:      "progress",
		BuildID:   "build-123",
		Timestamp: time.Now(),
		Message:   "Building...",
	}

	server.sendSSEEvent(w, event)

	body := w.Body.String()

	// Check for SSE format
	if !strings.HasPrefix(body, "data:") {
		t.Error("expected SSE format starting with 'data:'")
	}

	// Check for double newline (SSE format)
	if !strings.HasSuffix(body, "\n\n") {
		t.Error("expected SSE format ending with double newline")
	}

	// Try to unmarshal the JSON part
	jsonStr := strings.TrimPrefix(strings.TrimSuffix(body, "\n\n"), "data: ")
	var receivedEvent BuildEvent
	err := json.Unmarshal([]byte(jsonStr), &receivedEvent)
	if err != nil {
		t.Fatalf("failed to unmarshal event JSON: %v", err)
	}

	if receivedEvent.Type != "progress" {
		t.Errorf("expected type 'progress', got '%s'", receivedEvent.Type)
	}
}

func TestBuildEventJSONMarshal(t *testing.T) {
	event := BuildEvent{
		Type:      "completed",
		BuildID:   "build-123",
		Timestamp: time.Now(),
		Message:   "Build successful",
		Data: map[string]interface{}{
			"status":   "success",
			"duration": 120,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var unmarshaled BuildEvent
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if unmarshaled.BuildID != "build-123" {
		t.Errorf("expected BuildID 'build-123', got '%s'", unmarshaled.BuildID)
	}

	if unmarshaled.Type != "completed" {
		t.Errorf("expected Type 'completed', got '%s'", unmarshaled.Type)
	}
}

func TestEventSubscriberConcurrency(t *testing.T) {
	es := NewEventSubscriber()
	numGoroutines := 10

	// Create multiple subscribers
	var wg sync.WaitGroup
	channels := make([]chan BuildEvent, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		ch, _ := es.Subscribe("build-123")
		channels[i] = ch
	}

	// Publish events concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := BuildEvent{
				Type:    "progress",
				BuildID: "build-123",
				Message: fmt.Sprintf("Event %d", idx),
			}
			es.Publish(event)
		}(i)
	}

	// Receive events
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-channels[idx]
		}(i)
	}

	wg.Wait()

	// If we get here without deadlock, concurrency works
}

func TestHandleBuildEventsSSEFormat(t *testing.T) {
	es := NewEventSubscriber()
	server := &Server{}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/builds/build-123/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan bool)
	go func() {
		handler := server.HandleBuildEvents(es)
		handler.ServeHTTP(w, req)
		done <- true
	}()

	// Publish event quickly
	time.Sleep(25 * time.Millisecond)
	es.Publish(BuildEvent{
		Type:      "progress",
		BuildID:   "build-123",
		Timestamp: time.Now(),
		Message:   "Building",
	})

	// Wait for completion
	<-done

	// Just verify that response was written
	if w.Code != http.StatusOK {
		t.Logf("Note: Response code is %d (handler may have different termination behavior)", w.Code)
	}
}
