package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

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

// NewJSONStore creates a new JSON-based state store.
func NewJSONStore(dataDir string) foundation.Result[*JSONStore, error] {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return foundation.Err[*JSONStore, error](
			foundation.InternalError("failed to create data directory").
				WithCause(err).
				WithContext(foundation.Fields{"data_dir": dataDir}).
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

	// Load existing data
	if err := store.loadFromDisk(); err != nil {
		// Log warning but continue with empty state
		return foundation.Ok[*JSONStore, error](store)
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
func (js *JSONStore) WithTransaction(ctx context.Context, fn func(StateStore) error) foundation.Result[struct{}, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	if err := fn(js); err != nil {
		return foundation.Err[struct{}, error](err)
	}

	// Auto-save after transaction if enabled
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save after transaction").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

// Health returns the health status of the store.
func (js *JSONStore) Health(ctx context.Context) foundation.Result[StoreHealth, error] {
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
func (js *JSONStore) Close(ctx context.Context) foundation.Result[struct{}, error] {
	js.mu.Lock()
	defer js.mu.Unlock()

	// Perform final save
	if err := js.saveToDiskUnsafe(); err != nil {
		return foundation.Err[struct{}, error](
			foundation.InternalError("failed to save during close").
				WithCause(err).
				Build(),
		)
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

// loadFromDisk loads the state from JSON files.
func (js *JSONStore) loadFromDisk() error {
	statePath := filepath.Join(js.dataDir, "daemon-state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing state file
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	// Use the same format as the original StateManager for compatibility
	var legacyState struct {
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

	if err := json.Unmarshal(data, &legacyState); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Migrate data to new format
	if legacyState.Repositories != nil {
		js.repositories = legacyState.Repositories
	}
	if legacyState.Builds != nil {
		js.builds = legacyState.Builds
	}
	if legacyState.Schedules != nil {
		js.schedules = legacyState.Schedules
	}
	if legacyState.Statistics != nil {
		js.statistics = legacyState.Statistics
	}
	if legacyState.Configuration != nil {
		js.configuration = legacyState.Configuration
	}

	// Update daemon info
	js.daemonInfo.Version = legacyState.Version
	js.daemonInfo.StartTime = legacyState.StartTime
	js.daemonInfo.LastUpdate = legacyState.LastUpdate
	js.daemonInfo.Status = legacyState.Status

	return nil
}

// saveToDiskUnsafe saves the state to disk without acquiring the lock.
func (js *JSONStore) saveToDiskUnsafe() error {
	now := time.Now()
	js.daemonInfo.LastUpdate = now

	// Create legacy format for compatibility
	legacyState := struct {
		Version       string                 `json:"version"`
		StartTime     time.Time              `json:"start_time"`
		LastUpdate    time.Time              `json:"last_update"`
		Status        string                 `json:"status"`
		Repositories  map[string]*Repository `json:"repositories"`
		Builds        map[string]*Build      `json:"builds"`
		Schedules     map[string]*Schedule   `json:"schedules"`
		Statistics    *Statistics            `json:"statistics"`
		Configuration map[string]any         `json:"configuration,omitempty"`
	}{
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

	data, err := json.MarshalIndent(legacyState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := filepath.Join(js.dataDir, "daemon-state.json")
	tempPath := statePath + ".tmp"

	// Atomic write using temporary file
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		return fmt.Errorf("failed to replace state file: %w", err)
	}

	js.lastSaved = &now
	return nil
}

// calculateStorageSize calculates the total storage size used by the store.
func (js *JSONStore) calculateStorageSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(js.dataDir, func(path string, info os.FileInfo, err error) error {
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

// jsonRepositoryStore implements RepositoryStore for the JSON store.
type jsonRepositoryStore struct {
	store *JSONStore
}

func (rs *jsonRepositoryStore) Create(ctx context.Context, repo *Repository) foundation.Result[*Repository, error] {
	if repo == nil {
		return foundation.Err[*Repository, error](
			foundation.ValidationError("repository cannot be nil").Build(),
		)
	}

	// Validate the repository
	if validationResult := repo.Validate(); !validationResult.Valid {
		return foundation.Err[*Repository, error](validationResult.ToError())
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	// Check if repository already exists
	if _, exists := rs.store.repositories[repo.URL]; exists {
		return foundation.Err[*Repository, error](
			foundation.ValidationError("repository already exists").
				WithContext(foundation.Fields{"url": repo.URL}).
				Build(),
		)
	}

	// Set timestamps
	now := time.Now()
	repo.CreatedAt = now
	repo.UpdatedAt = now

	// Store the repository
	rs.store.repositories[repo.URL] = repo

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			// Remove from memory if save failed
			delete(rs.store.repositories, repo.URL)
			return foundation.Err[*Repository, error](
				foundation.InternalError("failed to save repository").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[*Repository, error](repo)
}

func (rs *jsonRepositoryStore) GetByURL(ctx context.Context, url string) foundation.Result[foundation.Option[*Repository], error] {
	if url == "" {
		return foundation.Err[foundation.Option[*Repository], error](
			foundation.ValidationError("URL cannot be empty").Build(),
		)
	}

	rs.store.mu.RLock()
	defer rs.store.mu.RUnlock()

	if repo, exists := rs.store.repositories[url]; exists {
		// Return a copy to prevent external modification
		repoCopy := *repo
		return foundation.Ok[foundation.Option[*Repository], error](foundation.Some(&repoCopy))
	}

	return foundation.Ok[foundation.Option[*Repository], error](foundation.None[*Repository]())
}

func (rs *jsonRepositoryStore) Update(ctx context.Context, repo *Repository) foundation.Result[*Repository, error] {
	if repo == nil {
		return foundation.Err[*Repository, error](
			foundation.ValidationError("repository cannot be nil").Build(),
		)
	}

	// Validate the repository
	if validationResult := repo.Validate(); !validationResult.Valid {
		return foundation.Err[*Repository, error](validationResult.ToError())
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	// Check if repository exists
	if _, exists := rs.store.repositories[repo.URL]; !exists {
		return foundation.Err[*Repository, error](
			foundation.NotFoundError("repository").
				WithContext(foundation.Fields{"url": repo.URL}).
				Build(),
		)
	}

	// Update timestamp
	repo.UpdatedAt = time.Now()

	// Store the updated repository
	rs.store.repositories[repo.URL] = repo

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Repository, error](
				foundation.InternalError("failed to save repository update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[*Repository, error](repo)
}

func (rs *jsonRepositoryStore) List(ctx context.Context) foundation.Result[[]Repository, error] {
	rs.store.mu.RLock()
	defer rs.store.mu.RUnlock()

	repositories := make([]Repository, 0, len(rs.store.repositories))
	for _, repo := range rs.store.repositories {
		repositories = append(repositories, *repo)
	}

	// Sort by name for consistent ordering
	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].Name < repositories[j].Name
	})

	return foundation.Ok[[]Repository, error](repositories)
}

func (rs *jsonRepositoryStore) Delete(ctx context.Context, url string) foundation.Result[struct{}, error] {
	if url == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("URL cannot be empty").Build(),
		)
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	// Check if repository exists
	if _, exists := rs.store.repositories[url]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("repository").
				WithContext(foundation.Fields{"url": url}).
				Build(),
		)
	}

	// Remove the repository
	delete(rs.store.repositories, url)

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save repository deletion").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) IncrementBuildCount(ctx context.Context, url string, success bool) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		// Create repository if it doesn't exist (same behavior as original)
		name := url
		if slash := len(url) - 1; slash >= 0 {
			for i := slash; i >= 0; i-- {
				if url[i] == '/' {
					name = url[i+1:]
					break
				}
			}
		}
		if name != url && len(name) > 4 && name[len(name)-4:] == ".git" {
			name = name[:len(name)-4]
		}

		repo = &Repository{
			URL:       url,
			Name:      name,
			Branch:    "main", // default
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		rs.store.repositories[url] = repo
	}

	// Update counters
	now := time.Now()
	repo.LastBuild = foundation.Some(now)
	repo.BuildCount++
	if !success {
		repo.ErrorCount++
	}
	repo.UpdatedAt = now

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save build count update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocumentCount(ctx context.Context, url string, count int) foundation.Result[struct{}, error] {
	if count < 0 {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("document count cannot be negative").Build(),
		)
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		// Create repository if it doesn't exist
		name := url
		if slash := len(url) - 1; slash >= 0 {
			for i := slash; i >= 0; i-- {
				if url[i] == '/' {
					name = url[i+1:]
					break
				}
			}
		}
		if name != url && len(name) > 4 && name[len(name)-4:] == ".git" {
			name = name[:len(name)-4]
		}

		repo = &Repository{
			URL:       url,
			Name:      name,
			Branch:    "main",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		rs.store.repositories[url] = repo
	}

	repo.DocumentCount = count
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save document count update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocFilesHash(ctx context.Context, url string, hash string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("repository").
				WithContext(foundation.Fields{"url": url}).
				Build(),
		)
	}

	repo.DocFilesHash = foundation.Some(hash)
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save doc files hash update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocFilePaths(ctx context.Context, url string, paths []string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("repository").
				WithContext(foundation.Fields{"url": url}).
				Build(),
		)
	}

	// Make a copy of the paths to prevent external modification
	repo.DocFilePaths = append([]string{}, paths...)
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save doc file paths update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

// jsonBuildStore implements BuildStore for the JSON store.
type jsonBuildStore struct {
	store *JSONStore
}

func (bs *jsonBuildStore) Create(ctx context.Context, build *Build) foundation.Result[*Build, error] {
	if build == nil {
		return foundation.Err[*Build, error](
			foundation.ValidationError("build cannot be nil").Build(),
		)
	}

	if validationResult := build.Validate(); !validationResult.Valid {
		return foundation.Err[*Build, error](validationResult.ToError())
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	now := time.Now()
	build.CreatedAt = now
	build.UpdatedAt = now

	bs.store.builds[build.ID] = build

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			delete(bs.store.builds, build.ID)
			return foundation.Err[*Build, error](
				foundation.InternalError("failed to save build").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Build, error](build)
}

func (bs *jsonBuildStore) GetByID(ctx context.Context, id string) foundation.Result[foundation.Option[*Build], error] {
	if id == "" {
		return foundation.Err[foundation.Option[*Build], error](
			foundation.ValidationError("ID cannot be empty").Build(),
		)
	}

	bs.store.mu.RLock()
	defer bs.store.mu.RUnlock()

	if build, exists := bs.store.builds[id]; exists {
		buildCopy := *build
		return foundation.Ok[foundation.Option[*Build], error](foundation.Some(&buildCopy))
	}

	return foundation.Ok[foundation.Option[*Build], error](foundation.None[*Build]())
}

func (bs *jsonBuildStore) Update(ctx context.Context, build *Build) foundation.Result[*Build, error] {
	if build == nil {
		return foundation.Err[*Build, error](
			foundation.ValidationError("build cannot be nil").Build(),
		)
	}

	if validationResult := build.Validate(); !validationResult.Valid {
		return foundation.Err[*Build, error](validationResult.ToError())
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	if _, exists := bs.store.builds[build.ID]; !exists {
		return foundation.Err[*Build, error](
			foundation.NotFoundError("build").
				WithContext(foundation.Fields{"id": build.ID}).
				Build(),
		)
	}

	build.UpdatedAt = time.Now()
	bs.store.builds[build.ID] = build

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Build, error](
				foundation.InternalError("failed to save build update").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Build, error](build)
}

func (bs *jsonBuildStore) List(ctx context.Context, opts ListOptions) foundation.Result[[]Build, error] {
	bs.store.mu.RLock()
	defer bs.store.mu.RUnlock()

	builds := make([]Build, 0, len(bs.store.builds))
	for _, build := range bs.store.builds {
		builds = append(builds, *build)
	}

	// Sort by creation time (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].CreatedAt.After(builds[j].CreatedAt)
	})

	// Apply pagination if specified
	if opts.Limit.IsSome() && opts.Limit.Unwrap() > 0 {
		start := 0
		if opts.Offset.IsSome() {
			start = opts.Offset.Unwrap()
		}

		if start > len(builds) {
			start = len(builds)
		}

		end := start + opts.Limit.Unwrap()
		if end > len(builds) {
			end = len(builds)
		}

		builds = builds[start:end]
	}

	return foundation.Ok[[]Build, error](builds)
}

func (bs *jsonBuildStore) Delete(ctx context.Context, id string) foundation.Result[struct{}, error] {
	if id == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("ID cannot be empty").Build(),
		)
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	if _, exists := bs.store.builds[id]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("build").
				WithContext(foundation.Fields{"id": id}).
				Build(),
		)
	}

	delete(bs.store.builds, id)

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save build deletion").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (bs *jsonBuildStore) Cleanup(ctx context.Context, maxBuilds int) foundation.Result[int, error] {
	if maxBuilds <= 0 {
		return foundation.Err[int, error](
			foundation.ValidationError("maxBuilds must be positive").Build(),
		)
	}

	bs.store.mu.Lock()
	defer bs.store.mu.Unlock()

	builds := make([]*Build, 0, len(bs.store.builds))
	for _, build := range bs.store.builds {
		builds = append(builds, build)
	}

	if len(builds) <= maxBuilds {
		return foundation.Ok[int, error](0) // No cleanup needed
	}

	// Sort by creation time (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].CreatedAt.After(builds[j].CreatedAt)
	})

	// Keep only the newest maxBuilds
	toDelete := builds[maxBuilds:]
	deletedCount := len(toDelete)

	for _, build := range toDelete {
		delete(bs.store.builds, build.ID)
	}

	if bs.store.autoSaveEnabled {
		if err := bs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[int, error](
				foundation.InternalError("failed to save build cleanup").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[int, error](deletedCount)
}

// jsonScheduleStore implements ScheduleStore for the JSON store.
type jsonScheduleStore struct {
	store *JSONStore
}

func (ss *jsonScheduleStore) Create(ctx context.Context, schedule *Schedule) foundation.Result[*Schedule, error] {
	if schedule == nil {
		return foundation.Err[*Schedule, error](
			foundation.ValidationError("schedule cannot be nil").Build(),
		)
	}

	if validationResult := schedule.Validate(); !validationResult.Valid {
		return foundation.Err[*Schedule, error](validationResult.ToError())
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	ss.store.schedules[schedule.ID] = schedule

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			delete(ss.store.schedules, schedule.ID)
			return foundation.Err[*Schedule, error](
				foundation.InternalError("failed to save schedule").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Schedule, error](schedule)
}

func (ss *jsonScheduleStore) GetByID(ctx context.Context, id string) foundation.Result[foundation.Option[*Schedule], error] {
	if id == "" {
		return foundation.Err[foundation.Option[*Schedule], error](
			foundation.ValidationError("ID cannot be empty").Build(),
		)
	}

	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	if schedule, exists := ss.store.schedules[id]; exists {
		scheduleCopy := *schedule
		return foundation.Ok[foundation.Option[*Schedule], error](foundation.Some(&scheduleCopy))
	}

	return foundation.Ok[foundation.Option[*Schedule], error](foundation.None[*Schedule]())
}

func (ss *jsonScheduleStore) Update(ctx context.Context, schedule *Schedule) foundation.Result[*Schedule, error] {
	if schedule == nil {
		return foundation.Err[*Schedule, error](
			foundation.ValidationError("schedule cannot be nil").Build(),
		)
	}

	if validationResult := schedule.Validate(); !validationResult.Valid {
		return foundation.Err[*Schedule, error](validationResult.ToError())
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	if _, exists := ss.store.schedules[schedule.ID]; !exists {
		return foundation.Err[*Schedule, error](
			foundation.NotFoundError("schedule").
				WithContext(foundation.Fields{"id": schedule.ID}).
				Build(),
		)
	}

	schedule.UpdatedAt = time.Now()
	ss.store.schedules[schedule.ID] = schedule

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Schedule, error](
				foundation.InternalError("failed to save schedule update").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Schedule, error](schedule)
}

func (ss *jsonScheduleStore) Delete(ctx context.Context, id string) foundation.Result[struct{}, error] {
	if id == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("ID cannot be empty").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	if _, exists := ss.store.schedules[id]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("schedule").
				WithContext(foundation.Fields{"id": id}).
				Build(),
		)
	}

	delete(ss.store.schedules, id)

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save schedule deletion").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (ss *jsonScheduleStore) List(ctx context.Context) foundation.Result[[]Schedule, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	schedules := make([]Schedule, 0, len(ss.store.schedules))
	for _, schedule := range ss.store.schedules {
		schedules = append(schedules, *schedule)
	}

	// Sort by next run time
	sort.Slice(schedules, func(i, j int) bool {
		// Handle Option[time.Time] properly
		if !schedules[i].NextRun.IsSome() {
			return false
		}
		if !schedules[j].NextRun.IsSome() {
			return true
		}
		return schedules[i].NextRun.Unwrap().Before(schedules[j].NextRun.Unwrap())
	})

	return foundation.Ok[[]Schedule, error](schedules)
}

func (ss *jsonScheduleStore) GetActive(ctx context.Context) foundation.Result[[]Schedule, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	schedules := make([]Schedule, 0)
	for _, schedule := range ss.store.schedules {
		// Check if schedule is active
		if schedule.IsActive {
			schedules = append(schedules, *schedule)
		}
	}

	// Sort by next run time
	sort.Slice(schedules, func(i, j int) bool {
		// Handle Option[time.Time] properly
		if !schedules[i].NextRun.IsSome() {
			return false
		}
		if !schedules[j].NextRun.IsSome() {
			return true
		}
		return schedules[i].NextRun.Unwrap().Before(schedules[j].NextRun.Unwrap())
	})

	return foundation.Ok[[]Schedule, error](schedules)
}

// jsonStatisticsStore implements StatisticsStore for the JSON store.
type jsonStatisticsStore struct {
	store *JSONStore
}

func (ss *jsonStatisticsStore) Get(ctx context.Context) foundation.Result[*Statistics, error] {
	ss.store.mu.RLock()
	defer ss.store.mu.RUnlock()

	statsCopy := *ss.store.statistics
	return foundation.Ok[*Statistics, error](&statsCopy)
}

func (ss *jsonStatisticsStore) Update(ctx context.Context, stats *Statistics) foundation.Result[*Statistics, error] {
	if stats == nil {
		return foundation.Err[*Statistics, error](
			foundation.ValidationError("statistics cannot be nil").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	stats.LastUpdated = time.Now()
	ss.store.statistics = stats

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Statistics, error](
				foundation.InternalError("failed to save statistics").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*Statistics, error](stats)
}

func (ss *jsonStatisticsStore) RecordBuild(ctx context.Context, build *Build) foundation.Result[struct{}, error] {
	if build == nil {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("build cannot be nil").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	ss.store.statistics.TotalBuilds++
	if build.Status == BuildStatusCompleted {
		ss.store.statistics.SuccessfulBuilds++
	} else if build.Status == BuildStatusFailed {
		ss.store.statistics.FailedBuilds++
	}
	ss.store.statistics.LastUpdated = time.Now()

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save build statistics").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (ss *jsonStatisticsStore) RecordDiscovery(ctx context.Context, documentCount int) foundation.Result[struct{}, error] {
	if documentCount < 0 {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("document count cannot be negative").Build(),
		)
	}

	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	ss.store.statistics.TotalDiscoveries++
	ss.store.statistics.DocumentsFound += int64(documentCount)
	ss.store.statistics.LastUpdated = time.Now()

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save discovery statistics").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (ss *jsonStatisticsStore) Reset(ctx context.Context) foundation.Result[struct{}, error] {
	ss.store.mu.Lock()
	defer ss.store.mu.Unlock()

	now := time.Now()
	ss.store.statistics = &Statistics{
		LastStatReset: now,
		LastUpdated:   now,
	}

	if ss.store.autoSaveEnabled {
		if err := ss.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save statistics reset").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

// jsonConfigurationStore implements ConfigurationStore for the JSON store.
type jsonConfigurationStore struct {
	store *JSONStore
}

func (cs *jsonConfigurationStore) Get(ctx context.Context, key string) foundation.Result[foundation.Option[any], error] {
	if key == "" {
		return foundation.Err[foundation.Option[any], error](
			foundation.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.RLock()
	defer cs.store.mu.RUnlock()

	if value, exists := cs.store.configuration[key]; exists {
		return foundation.Ok[foundation.Option[any], error](foundation.Some(value))
	}

	return foundation.Ok[foundation.Option[any], error](foundation.None[any]())
}

func (cs *jsonConfigurationStore) Set(ctx context.Context, key string, value any) foundation.Result[struct{}, error] {
	if key == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.Lock()
	defer cs.store.mu.Unlock()

	cs.store.configuration[key] = value

	if cs.store.autoSaveEnabled {
		if err := cs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save configuration").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (cs *jsonConfigurationStore) Delete(ctx context.Context, key string) foundation.Result[struct{}, error] {
	if key == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("key cannot be empty").Build(),
		)
	}

	cs.store.mu.Lock()
	defer cs.store.mu.Unlock()

	// Check if key exists
	if _, exists := cs.store.configuration[key]; !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("configuration key").
				WithContext(foundation.Fields{"key": key}).
				Build(),
		)
	}

	delete(cs.store.configuration, key)

	if cs.store.autoSaveEnabled {
		if err := cs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save configuration deletion").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (cs *jsonConfigurationStore) List(ctx context.Context) foundation.Result[map[string]any, error] {
	return cs.GetAll(ctx)
}

func (cs *jsonConfigurationStore) GetAll(ctx context.Context) foundation.Result[map[string]any, error] {
	cs.store.mu.RLock()
	defer cs.store.mu.RUnlock()

	// Return a deep copy to prevent external modification
	result := make(map[string]any, len(cs.store.configuration))
	for k, v := range cs.store.configuration {
		result[k] = v
	}

	return foundation.Ok[map[string]any, error](result)
}

// jsonDaemonInfoStore implements DaemonInfoStore for the JSON store.
type jsonDaemonInfoStore struct {
	store *JSONStore
}

func (ds *jsonDaemonInfoStore) Get(ctx context.Context) foundation.Result[*DaemonInfo, error] {
	ds.store.mu.RLock()
	defer ds.store.mu.RUnlock()

	infoCopy := *ds.store.daemonInfo
	return foundation.Ok[*DaemonInfo, error](&infoCopy)
}

func (ds *jsonDaemonInfoStore) Update(ctx context.Context, info *DaemonInfo) foundation.Result[*DaemonInfo, error] {
	if info == nil {
		return foundation.Err[*DaemonInfo, error](
			foundation.ValidationError("daemon info cannot be nil").Build(),
		)
	}

	ds.store.mu.Lock()
	defer ds.store.mu.Unlock()

	info.LastUpdate = time.Now()
	ds.store.daemonInfo = info

	if ds.store.autoSaveEnabled {
		if err := ds.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*DaemonInfo, error](
				foundation.InternalError("failed to save daemon info").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[*DaemonInfo, error](info)
}

func (ds *jsonDaemonInfoStore) UpdateStatus(ctx context.Context, status string) foundation.Result[struct{}, error] {
	if status == "" {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("status cannot be empty").Build(),
		)
	}

	ds.store.mu.Lock()
	defer ds.store.mu.Unlock()

	ds.store.daemonInfo.Status = status
	ds.store.daemonInfo.LastUpdate = time.Now()

	if ds.store.autoSaveEnabled {
		if err := ds.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save daemon status").WithCause(err).Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}
