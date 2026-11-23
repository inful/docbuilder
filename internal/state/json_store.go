package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

const healthyStatus = "healthy"

// JSONStore implements StateStore using JSON file persistence.
// This demonstrates how to implement the state interfaces while maintaining
// the same persistence format as the original StateManager.
type JSONStore struct {
	dataDir       string
	mu            sync.RWMutex
	repositories  map[string]*Repository
	schedules     map[string]*Schedule
	statistics    *Statistics
	configuration map[string]any
	daemonInfo    *DaemonInfo
	lastSaved     *time.Time
	// dirty records that the state changed and that the file has to be
	// written. The changes are written when the state is saved: a build
	// writes the state of every repository, and writing the whole file
	// for each of them rewrote it hundreds of times.
	dirty bool
}

const stateSnapshotFormatVersion = "1"

// stateSnapshot is the typed on-disk representation for the JSON store.
// Additional fields should be added here so load/save logic remains centralized.
type stateSnapshot struct {
	FormatVersion string                 `json:"format_version,omitempty"`
	Version       string                 `json:"version"`
	StartTime     time.Time              `json:"start_time"`
	LastUpdate    time.Time              `json:"last_update"`
	Status        string                 `json:"status"`
	Repositories  map[string]*Repository `json:"repositories"`
	Schedules     map[string]*Schedule   `json:"schedules"`
	Statistics    *Statistics            `json:"statistics"`
	Configuration map[string]any         `json:"configuration,omitempty"`
}

// NewJSONStore creates a new JSON-based state store.
func NewJSONStore(dataDir string) foundation.Result[*JSONStore, error] {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return foundation.Err[*JSONStore, error](
			errors.InternalError("failed to create data directory").
				WithCause(err).
				WithContext("data_dir", dataDir).
				Build(),
		)
	}

	store := &JSONStore{
		dataDir:       dataDir,
		repositories:  make(map[string]*Repository),
		schedules:     make(map[string]*Schedule),
		configuration: make(map[string]any),
		statistics: &Statistics{
			LastStatReset: time.Now(),
			LastUpdated:   time.Now(),
		},
		daemonInfo: &DaemonInfo{
			Version:    "2.0.0",
			StartTime:  time.Now(),
			LastUpdate: time.Now(),
			Status:     "starting",
		},
	}

	if err := store.loadFromDisk(); err != nil {
		// An unreadable state file must not brick the daemon. Move it aside and
		// continue with empty state; the original file is preserved next to it.
		quarantinePath, quarantineErr := store.quarantineStateFile()
		if quarantineErr != nil {
			slog.Warn("daemon state could not be loaded and could not be quarantined; continuing with empty state",
				"error", err, "quarantine_error", quarantineErr, "data_dir", dataDir)
		} else {
			slog.Warn("daemon state could not be loaded; continuing with empty state",
				"error", err, "quarantined_path", quarantinePath, "data_dir", dataDir)
		}
	}

	return foundation.Ok[*JSONStore, error](store)
}

// stateFilePath returns the path of the on-disk state snapshot.
func (js *JSONStore) stateFilePath() string {
	return filepath.Join(js.dataDir, "daemon-state.json")
}

// quarantineStateFile moves an unreadable state file aside so the daemon can
// start with empty state without destroying the original data. The returned
// path is the new location of the quarantined file.
func (js *JSONStore) quarantineStateFile() (string, error) {
	statePath := js.stateFilePath()
	quarantinePath := fmt.Sprintf("%s.corrupt-%s", statePath, time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.Rename(statePath, quarantinePath); err != nil {
		return "", err
	}
	return quarantinePath, nil
}

// Repositories returns the repository store interface.
func (js *JSONStore) Repositories() RepositoryStore {
	return &jsonRepositoryStore{store: js}
}

// Schedules returns the schedule store interface.
func (js *JSONStore) Schedules() ScheduleStore {
	return &jsonScheduleStore{store: js}
}

// Statistics returns the statistics store interface.
func (js *JSONStore) Statistics() StatisticsStore {
	return &jsonStatisticsStore{store: js}
}

// Configuration returns the configuration store interface.
func (js *JSONStore) Configuration() ConfigurationStore {
	return &jsonConfigurationStore{store: js}
}

// DaemonInfo returns the daemon info store interface.
func (js *JSONStore) DaemonInfo() DaemonInfoStore {
	return &jsonDaemonInfoStore{store: js}
}

// Health returns the health status of the store.
func (js *JSONStore) Health(_ context.Context) foundation.Result[StoreHealth, error] {
	js.mu.RLock()
	defer js.mu.RUnlock()

	health := StoreHealth{
		Status:    healthyStatus,
		CheckedAt: time.Now(),
	}

	// Check if we can access the data directory
	if _, err := os.Stat(js.dataDir); err != nil {
		health.Status = "unhealthy"
		health.Message = fmt.Sprintf("cannot access data directory: %v", err)
	}

	if js.lastSaved != nil {
		health.LastBackup = js.lastSaved
	}

	return foundation.Ok[StoreHealth, error](health)
}

// Flush writes the state to disk.
//
// The store writes on every change, so a flush is only needed when the
// caller wants to be sure: the state reached the disk and the write
// worked.
func (js *JSONStore) Flush(_ context.Context) foundation.Result[struct{}, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	if !js.dirty {
		// Nothing changed since the last write.
		return foundation.Ok[struct{}, error](struct{}{})
	}

	if err := js.saveToDiskUnsafe(); err != nil {
		return foundation.Err[struct{}, error](
			errors.InternalError("failed to flush state").
				WithCause(err).
				Build(),
		)
	}
	return foundation.Ok[struct{}, error](struct{}{})
}

// Close gracefully shuts down the store.
func (js *JSONStore) Close(_ context.Context) foundation.Result[struct{}, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	// Perform final save
	if err := js.saveToDiskUnsafe(); err != nil {
		return foundation.Err[struct{}, error](
			errors.InternalError("failed to save during close").
				WithCause(err).
				Build(),
		)
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

// loadFromDisk loads the state from JSON files.
func (js *JSONStore) loadFromDisk() error {
	statePath := js.stateFilePath()

	// #nosec G304 - statePath is internal, dataDir is controlled by application
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing state file
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	snapshot, err := decodeStateSnapshot(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}
	js.applySnapshot(snapshot)
	return nil
}

// saveToDiskUnsafe saves the state to disk without acquiring the lock.
func (js *JSONStore) saveToDiskUnsafe() error {
	now := time.Now()
	js.daemonInfo.LastUpdate = now

	snapshot := js.snapshot()
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := js.stateFilePath()
	tempPath := statePath + ".tmp"

	// Atomic write using temporary file
	// #nosec G306 -- state file needs to be readable by the process, 0644 is acceptable
	// #nosec G304 -- the path is the state file of the data directory
	// The state file holds the URL of the repositories, and the URL can
	// hold a token. The file is private to its owner.
	//nolint:gosec // the state file is read by the process, 0600 is acceptable
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open temporary state file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}
	// The data is on the disk before the file is put in place: a crash
	// after the rename but before the data reached the disk would leave
	// a state file that is empty or half written.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to sync temporary state file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temporary state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		return fmt.Errorf("failed to replace state file: %w", err)
	}

	// The directory is synced as well: the rename itself has to reach
	// the disk, or a crash brings the old file back.
	if dir, err := os.Open(filepath.Dir(statePath)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	js.lastSaved = &now
	js.dirty = false
	return nil
}

func decodeStateSnapshot(data []byte) (stateSnapshot, error) {
	var snapshot stateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return stateSnapshot{}, err
	}
	// Auto-upgrade legacy state files written before format_version was added.
	// The next saveToDiskUnsafe call will persist the upgraded field, so this
	// is a one-time migration on first load. Files with an explicit
	// format_version that doesn't match the current one still fail below.
	if snapshot.FormatVersion == "" {
		snapshot.FormatVersion = stateSnapshotFormatVersion
	}
	if snapshot.FormatVersion != stateSnapshotFormatVersion {
		return stateSnapshot{}, fmt.Errorf("unsupported state snapshot format_version %q (expected %s)", snapshot.FormatVersion, stateSnapshotFormatVersion)
	}
	return snapshot, nil
}

func (js *JSONStore) applySnapshot(snapshot stateSnapshot) {
	if snapshot.Repositories != nil {
		js.repositories = snapshot.Repositories
	} else if js.repositories == nil {
		js.repositories = make(map[string]*Repository)
	}
	if snapshot.Schedules != nil {
		js.schedules = snapshot.Schedules
	} else if js.schedules == nil {
		js.schedules = make(map[string]*Schedule)
	}
	if snapshot.Statistics != nil {
		js.statistics = snapshot.Statistics
	}
	if snapshot.Configuration != nil {
		js.configuration = snapshot.Configuration
	}
	if snapshot.Version != "" {
		js.daemonInfo.Version = snapshot.Version
	}
	if !snapshot.StartTime.IsZero() {
		js.daemonInfo.StartTime = snapshot.StartTime
	}
	if !snapshot.LastUpdate.IsZero() {
		js.daemonInfo.LastUpdate = snapshot.LastUpdate
	}
	if snapshot.Status != "" {
		js.daemonInfo.Status = snapshot.Status
	}
}

func (js *JSONStore) snapshot() stateSnapshot {
	return stateSnapshot{
		FormatVersion: stateSnapshotFormatVersion,
		Version:       js.daemonInfo.Version,
		StartTime:     js.daemonInfo.StartTime,
		LastUpdate:    js.daemonInfo.LastUpdate,
		Status:        js.daemonInfo.Status,
		Repositories:  js.repositories,
		Schedules:     js.schedules,
		Statistics:    js.statistics,
		Configuration: js.configuration,
	}
}
