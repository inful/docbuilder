package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// BuildEvent represents a build event for streaming.
type BuildEvent struct {
	Type      string      `json:"type"` // connected, progress, completed, failed, error
	BuildID   string      `json:"build_id"`
	Timestamp time.Time   `json:"timestamp"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// EventSubscriber manages build event subscriptions.
type EventSubscriber struct {
	subscribers map[string][]chan BuildEvent
	mu          sync.RWMutex
}

// NewEventSubscriber creates a new event subscriber.
func NewEventSubscriber() *EventSubscriber {
	return &EventSubscriber{
		subscribers: make(map[string][]chan BuildEvent),
	}
}

// Subscribe creates a subscription channel for a build's events.
// Returns a channel that will receive events and a function to unsubscribe.
func (es *EventSubscriber) Subscribe(buildID string) (chan BuildEvent, func()) {
	es.mu.Lock()
	defer es.mu.Unlock()

	ch := make(chan BuildEvent, 10) // Buffered channel to prevent blocking
	es.subscribers[buildID] = append(es.subscribers[buildID], ch)

	unsubscribe := func() {
		es.mu.Lock()
		defer es.mu.Unlock()

		// Remove this channel from subscribers
		if subs, ok := es.subscribers[buildID]; ok {
			for i, sub := range subs {
				if sub == ch {
					// Remove by replacing with last element and truncating
					es.subscribers[buildID] = append(subs[:i], subs[i+1:]...)
					close(ch)
					break
				}
			}

			// Remove build ID if no more subscribers
			if len(es.subscribers[buildID]) == 0 {
				delete(es.subscribers, buildID)
			}
		}
	}

	return ch, unsubscribe
}

// Publish sends an event to all subscribers of a build.
func (es *EventSubscriber) Publish(event BuildEvent) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if subs, ok := es.subscribers[event.BuildID]; ok {
		for _, ch := range subs {
			// Non-blocking send to avoid blocking publisher
			select {
			case ch <- event:
			default:
				// Channel full, skip this subscriber
				slog.Warn("Event channel full, dropping event", "build_id", event.BuildID)
			}
		}
	}
}

// GetSubscriberCount returns the number of active subscriptions for a build.
func (es *EventSubscriber) GetSubscriberCount(buildID string) int {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return len(es.subscribers[buildID])
}

// EventStream represents an SSE stream context.
type EventStream struct {
	BuildID  string
	EventCh  chan BuildEvent
	Timeout  time.Duration
	MaxRetry int
}

// HandleBuildEvents handles SSE connections for build events.
func (s *Server) HandleBuildEvents(eventSubscriber *EventSubscriber) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buildID := chi.URLParam(r, "id")
		if buildID == "" {
			s.Error(w, r, http.StatusBadRequest, "missing build ID")
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

		// Subscribe to events
		eventCh, unsubscribe := eventSubscriber.Subscribe(buildID)
		defer unsubscribe()

		slog.Info("Build event stream opened", "build_id", buildID)

		// Send initial connection event
		connEvent := BuildEvent{
			Type:      "connected",
			BuildID:   buildID,
			Timestamp: time.Now(),
			Message:   "Connected to build event stream",
		}
		s.sendSSEEvent(w, connEvent)

		// Check for client disconnect
		ctx := r.Context()
		done := ctx.Done()

		// Event streaming loop with timeout
		timeout := time.After(60 * time.Second) // 60-second timeout per event batch

		for {
			select {
			case <-done:
				// Client disconnected
				slog.Info("Build event stream closed (client disconnect)", "build_id", buildID)
				return

			case <-timeout:
				// Timeout without events
				slog.Debug("Build event stream timeout", "build_id", buildID)
				s.sendSSEEvent(w, BuildEvent{
					Type:      "timeout",
					BuildID:   buildID,
					Timestamp: time.Now(),
					Message:   "No events received within timeout period",
				})
				return

			case event, ok := <-eventCh:
				if !ok {
					// Channel closed
					slog.Info("Build event stream closed (channel closed)", "build_id", buildID)
					return
				}

				s.sendSSEEvent(w, event)

				// Reset timeout on successful event
				timeout = time.After(60 * time.Second)

				// Close stream on terminal events
				if event.Type == "completed" || event.Type == "failed" {
					slog.Info("Build event stream closed (terminal event)", "build_id", buildID, "type", event.Type)
					return
				}
			}
		}
	}
}

// sendSSEEvent sends an event in SSE format.
func (s *Server) sendSSEEvent(w http.ResponseWriter, event BuildEvent) {
	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal SSE event", "error", err)
		return
	}

	// Write SSE-formatted event
	fmt.Fprintf(w, "data: %s\n\n", eventJSON)

	// Flush to ensure immediate delivery
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	} else {
		slog.Warn("Response writer does not support flushing")
	}
}

// PublishBuildEvent publishes a build event to all subscribers.
func (s *Server) PublishBuildEvent(subscriber *EventSubscriber, event BuildEvent) {
	subscriber.Publish(event)
}

// MockEventPublisher is a test helper for publishing events.
type MockEventPublisher struct {
	Events []BuildEvent
}

// Publish appends an event to the mock publisher's list.
func (m *MockEventPublisher) Publish(event BuildEvent) {
	m.Events = append(m.Events, event)
}
