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

	// GetRecent retrieves the most recent events, oldest first. The limit
	// keeps a replay from reading the whole log.
	GetRecent(ctx context.Context, limit int) ([]Event, error)

	// Prune deletes the events older than the given time and returns how
	// many were removed. Without it the log grows without bound.
	Prune(ctx context.Context, olderThan time.Time) (int64, error)

	// Close closes the store and releases resources.
	Close() error
}
