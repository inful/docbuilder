package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

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
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initialize(); err != nil {
		_ = db.Close() // Best effort cleanup on initialization error
		return nil, fmt.Errorf("initialize schema: %w", err)
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
	_, err := s.db.Exec(schema)
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
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	timestamp := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO events (build_id, event_type, timestamp, payload, metadata) VALUES (?, ?, ?, ?, ?)",
		buildID, eventType, timestamp, payload, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
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
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

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
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("scan event: %w", err)
		}

		e.EventTimestamp = time.Unix(timestampUnix, 0)

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &e.EventMetadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}

		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return events, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}
