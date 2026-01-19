package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-based event store.
// Use ":memory:" for in-memory database, or a file path for persistent storage.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, errors.WrapError(err, errors.CategoryEventStore, "could not open event store database").
			WithContext("path", dbPath).
			WithCause(ErrDatabaseOpenFailed).
			Build()
	}

	store := &SQLiteStore{db: db}
	if err := store.initialize(); err != nil {
		_ = db.Close() // Best effort cleanup on initialization error
		return nil, errors.WrapError(err, errors.CategoryEventStore, "failed to initialize event store schema").
			WithCause(ErrInitializeSchemaFailed).
			Build()
	}

	return store, nil
}

func (s *SQLiteStore) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		build_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		payload BLOB NOT NULL,
		metadata TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_build_id ON events(build_id);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_event_type ON events(event_type);
	`
	_, err := s.db.ExecContext(context.Background(), schema)
	return err
}

// Append adds a new event to the store.
func (s *SQLiteStore) Append(ctx context.Context, buildID, eventType string, payload []byte, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metadataJSON []byte
	if metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return errors.WrapError(err, errors.CategoryEventStore, "failed to marshal metadata").
				WithContext("build_id", buildID).
				WithContext("event_type", eventType).
				WithCause(ErrMarshalPayloadFailed).
				Build()
		}
	}

	timestamp := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO events (build_id, event_type, timestamp, payload, metadata) VALUES (?, ?, ?, ?, ?)",
		buildID, eventType, timestamp, payload, metadataJSON,
	)
	if err != nil {
		return errors.WrapError(err, errors.CategoryEventStore, "failed to insert event").
			WithContext("build_id", buildID).
			WithContext("event_type", eventType).
			WithCause(ErrEventAppendFailed).
			Build()
	}

	return nil
}

// GetByBuildID retrieves all events for a specific build.
func (s *SQLiteStore) GetByBuildID(ctx context.Context, buildID string) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, build_id, event_type, timestamp, payload, metadata FROM events WHERE build_id = ? ORDER BY id",
		buildID,
	)
	if err != nil {
		return nil, errors.WrapError(err, errors.CategoryEventStore, "failed to query events").
			WithContext("build_id", buildID).
			WithCause(ErrEventQueryFailed).
			Build()
	}
	defer func() { _ = rows.Close() }()

	return s.scanEvents(rows)
}

// GetRange retrieves events within a time range.
func (s *SQLiteStore) GetRange(ctx context.Context, start, end time.Time) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, build_id, event_type, timestamp, payload, metadata FROM events WHERE timestamp >= ? AND timestamp <= ? ORDER BY id",
		start.Unix(), end.Unix(),
	)
	if err != nil {
		return nil, errors.WrapError(err, errors.CategoryEventStore, "failed to query events by range").
			WithContext("start", start).
			WithContext("end", end).
			WithCause(ErrEventQueryFailed).
			Build()
	}
	defer func() { _ = rows.Close() }()

	return s.scanEvents(rows)
}

func (s *SQLiteStore) scanEvents(rows *sql.Rows) ([]Event, error) {
	var events []Event
	for rows.Next() {
		var e BaseEvent
		var timestampUnix int64
		var metadataJSON []byte

		err := rows.Scan(&e.EventID, &e.EventBuildID, &e.EventType, &timestampUnix, &e.EventPayload, &metadataJSON)
		if err != nil {
			return nil, errors.WrapError(err, errors.CategoryEventStore, "failed to scan event row").
				WithCause(ErrEventScanFailed).
				Build()
		}

		e.EventTimestamp = time.Unix(timestampUnix, 0)

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &e.EventMetadata); err != nil {
				return nil, errors.WrapError(err, errors.CategoryEventStore, "failed to unmarshal metadata").
					WithCause(ErrUnmarshalPayloadFailed).
					Build()
			}
		}

		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.WrapError(err, errors.CategoryEventStore, "error during event iteration").
			WithCause(ErrEventQueryFailed).
			Build()
	}

	return events, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}
