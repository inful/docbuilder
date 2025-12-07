package pipeline

import (
	"context"
	"sync"
)

// EventStore defines the interface for persisting events.
// This is a subset of eventstore.Store to avoid circular dependencies.
type EventStore interface {
	Append(ctx context.Context, buildID, eventType string, payload []byte, metadata map[string]string) error
}

// Handler processes an Event; return error to signal failure.
type Handler func(Event) error

// Bus is a simple synchronous pub/sub event bus.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
	eventStore  EventStore // optional event store for persistence
}

func NewBus() *Bus { return &Bus{subscribers: map[string][]Handler{}} }

// NewBusWithEventStore creates a bus that persists events to the store.
func NewBusWithEventStore(store EventStore) *Bus {
	return &Bus{
		subscribers: map[string][]Handler{},
		eventStore:  store,
	}
}

// Subscribe registers a handler for a given event name.
func (b *Bus) Subscribe(event string, h Handler) {
	if h == nil {
		return
	}
	b.mu.Lock()
	b.subscribers[event] = append(b.subscribers[event], h)
	b.mu.Unlock()
}

// Publish delivers an event to all handlers synchronously.
// If an event store is configured, the event is persisted before being delivered to handlers.
func (b *Bus) Publish(e Event) error {
	// Persist to event store if configured
	if b.eventStore != nil {
		// Extract buildID from event if available
		buildID := "unknown"
		if be, ok := e.(interface{ GetBuildID() string }); ok {
			buildID = be.GetBuildID()
		}

		// Persist event (use empty payload and metadata for now)
		ctx := context.Background()
		if err := b.eventStore.Append(ctx, buildID, e.Name(), []byte{}, nil); err != nil {
			// Log error but don't fail the build
			// In production, might want to use slog here
			_ = err
		}
	}

	// Execute handlers (existing behavior)
	b.mu.RLock()
	hs := append([]Handler(nil), b.subscribers[e.Name()]...)
	b.mu.RUnlock()
	for _, h := range hs {
		if err := h(e); err != nil {
			return err
		}
	}
	return nil
}
