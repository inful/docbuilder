package eventstore

import (
	"context"
	"time"
)

// Store defines the interface for persisting and retrieving events.
type Store interface {
	// Append adds a new event to the store.
	Append(ctx context.Context, buildID, eventType string, payload []byte, metadata map[string]string) error

	// GetByBuildID retrieves all events for a specific build.
	GetByBuildID(ctx context.Context, buildID string) ([]Event, error)

	// GetRange retrieves events within a time range.
	GetRange(ctx context.Context, start, end time.Time) ([]Event, error)

	// Close closes the store and releases resources.
	Close() error
}
