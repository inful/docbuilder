package pipeline

import (
	"context"
	"sync"
	"testing"
)

// MockEventStore implements EventStore for testing.
type MockEventStore struct {
	mu     sync.Mutex
	events []struct {
		buildID   string
		eventType string
		payload   []byte
		metadata  map[string]string
	}
}

func (m *MockEventStore) Append(ctx context.Context, buildID, eventType string, payload []byte, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, struct {
		buildID   string
		eventType string
		payload   []byte
		metadata  map[string]string
	}{buildID, eventType, payload, metadata})
	return nil
}

func (m *MockEventStore) EventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func (m *MockEventStore) LastEvent() (buildID, eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) == 0 {
		return "", ""
	}
	last := m.events[len(m.events)-1]
	return last.buildID, last.eventType
}

func TestBusWithoutEventStore(t *testing.T) {
	// Test that existing behavior works without event store
	bus := NewBus()

	called := false
	bus.Subscribe("TestEvent", func(e Event) error {
		called = true
		return nil
	})

	event := &SimpleEvent{E: "TestEvent"}
	if err := bus.Publish(event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if !called {
		t.Error("Handler was not called")
	}
}

func TestBusWithEventStore(t *testing.T) {
	// Event store persistence is now default behavior
	mockStore := &MockEventStore{}
	bus := NewBusWithEventStore(mockStore)

	handlerCalled := false
	bus.Subscribe("TestEvent", func(e Event) error {
		handlerCalled = true
		return nil
	})

	event := &SimpleEvent{E: "TestEvent"}
	if err := bus.Publish(event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify handler was called
	if !handlerCalled {
		t.Error("Handler was not called")
	}

	// Verify event was persisted to store
	if mockStore.EventCount() != 1 {
		t.Errorf("Expected 1 event in store, got %d", mockStore.EventCount())
	}

	buildID, eventType := mockStore.LastEvent()
	if eventType != "TestEvent" {
		t.Errorf("Expected event type TestEvent, got %s", eventType)
	}
	if buildID != "unknown" {
		t.Errorf("Expected buildID unknown, got %s", buildID)
	}
}

func TestBusBackwardCompatibility(t *testing.T) {
	// Test that bus without event store continues to work as before
	bus := NewBus()

	count := 0
	bus.Subscribe("TestEvent", func(e Event) error {
		count++
		return nil
	})
	bus.Subscribe("TestEvent", func(e Event) error {
		count++
		return nil
	})

	event := &SimpleEvent{E: "TestEvent"}
	if err := bus.Publish(event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 handlers called, got %d", count)
	}
}
