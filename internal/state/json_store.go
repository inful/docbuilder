package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// JSONStore implements StateStore using JSON file persistence.
// This demonstrates how to implement the state interfaces while maintaining
// the same persistence format as the original StateManager.
type JSONStore struct {
	dataDir         string
	mu              sync.RWMutex
	repositories    map[string]*Repository
	builds          map[string]*Build
	schedules       map[string]*Schedule
	statistics      *Statistics
	configuration   map[string]any
	daemonInfo      *DaemonInfo
	lastSaved       *time.Time
	autoSaveEnabled bool
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
	Builds        map[string]*Build      `json:"builds"`
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
		dataDir:         dataDir,
		repositories:    make(map[string]*Repository),
		builds:          make(map[string]*Build),
		schedules:       make(map[string]*Schedule),
		configuration:   make(map[string]any),
		autoSaveEnabled: true,
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
		return foundation.Err[*JSONStore, error](
			errors.InternalError("failed to load daemon state").
				WithCause(err).
				WithContext("data_dir", dataDir).
				Build(),
		)
	}

	return foundation.Ok[*JSONStore, error](store)
}

// Repositories returns the repository store interface.
func (js *JSONStore) Repositories() RepositoryStore {
	return &jsonRepositoryStore{store: js}
}

// Builds returns the build store interface.
func (js *JSONStore) Builds() BuildStore {
	return &jsonBuildStore{store: js}
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

// WithTransaction executes a function within a transaction-like context.
// For the JSON store, this uses a mutex to ensure consistency.
func (js *JSONStore) WithTransaction(_ context.Context, fn func(Store) error) foundation.Result[struct{}, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	if err := fn(js); err != nil {
		return foundation.Err[struct{}, error](err)
	}

	// Auto-save after transaction if enabled
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				errors.InternalError("failed to save after transaction").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

// Health returns the health status of the store.
func (js *JSONStore) Health(_ context.Context) foundation.Result[StoreHealth, error] {
	js.mu.RLock()
	defer js.mu.RUnlock()

	health := StoreHealth{
		Status:    "healthy",
		CheckedAt: time.Now(),
	}

	// Check if we can access the data directory
	if _, err := os.Stat(js.dataDir); err != nil {
		health.Status = "unhealthy"
		health.Message = fmt.Sprintf("cannot access data directory: %v", err)
	}

	// Add storage size if we can calculate it
	if health.Status == "healthy" {
		if size, err := js.calculateStorageSize(); err == nil {
			health.StorageSize = &size
		}
	}

	if js.lastSaved != nil {
		health.LastBackup = js.lastSaved
	}

	return foundation.Ok[StoreHealth, error](health)
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
	statePath := filepath.Join(js.dataDir, "daemon-state.json")

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

	statePath := filepath.Join(js.dataDir, "daemon-state.json")
	tempPath := statePath + ".tmp"

	// Atomic write using temporary file
	// #nosec G306 -- state file needs to be readable by the process, 0644 is acceptable
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		return fmt.Errorf("failed to replace state file: %w", err)
	}

	js.lastSaved = &now
	return nil
}

func decodeStateSnapshot(data []byte) (stateSnapshot, error) {
	var snapshot stateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return stateSnapshot{}, err
	}
	if snapshot.FormatVersion == "" {
		return stateSnapshot{}, fmt.Errorf("state snapshot missing format_version (legacy files are no longer supported)")
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
	if snapshot.Builds != nil {
		js.builds = snapshot.Builds
	} else if js.builds == nil {
		js.builds = make(map[string]*Build)
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
		Builds:        js.builds,
		Schedules:     js.schedules,
		Statistics:    js.statistics,
		Configuration: js.configuration,
	}
}

// calculateStorageSize calculates the total storage size used by the store.
func (js *JSONStore) calculateStorageSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(js.dataDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}
