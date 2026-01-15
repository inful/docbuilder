package eventstore

import (
	"bytes"
	"testing"
	"time"
)

func TestEventStoreAppendAndRetrieve(t *testing.T) {
	// Create in-memory store
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := t.Context()
	buildID := testBuildID
	eventType := "TestEvent"
	payload := []byte(`{"test": "data"}`)
	metadata := map[string]string{"key": "value"}

	// Test Append
	err = store.Append(ctx, buildID, eventType, payload, metadata)
	if err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	// Test GetByBuildID
	events, err := store.GetByBuildID(ctx, buildID)
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.BuildID() != buildID {
		t.Errorf("expected build_id %s, got %s", buildID, event.BuildID())
	}
	if event.Type() != eventType {
		t.Errorf("expected event_type %s, got %s", eventType, event.Type())
	}
	if !bytes.Equal(event.Payload(), payload) {
		t.Errorf("expected payload %s, got %s", payload, event.Payload())
	}
	if event.Metadata()["key"] != "value" {
		t.Errorf("expected metadata key=value, got %v", event.Metadata())
	}
}

func TestEventStoreGetRange(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := t.Context()
	now := time.Now()

	// Add events
	for range 3 {
		eventErr := store.Append(ctx, "build-1", "Event", []byte("data"), nil)
		if eventErr != nil {
			t.Fatalf("failed to append event: %v", eventErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Query range
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)
	events, err := store.GetRange(ctx, start, end)
	if err != nil {
		t.Fatalf("failed to get range: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestEventStoreMultipleBuilds(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := t.Context()

	// Add events for different builds
	_ = store.Append(ctx, "build-1", "Event1", []byte("data1"), nil)
	_ = store.Append(ctx, "build-2", "Event2", []byte("data2"), nil)
	_ = store.Append(ctx, "build-1", "Event3", []byte("data3"), nil)

	// Query build-1
	events, err := store.GetByBuildID(ctx, "build-1")
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events for build-1, got %d", len(events))
	}

	// Query build-2
	events, err = store.GetByBuildID(ctx, "build-2")
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 event for build-2, got %d", len(events))
	}
}
