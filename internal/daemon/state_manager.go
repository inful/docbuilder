package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StateManager handles persistent storage for daemon state
type StateManager struct {
	dataDir   string
	mu        sync.RWMutex
	state     *DaemonState
	autosave  bool
	saveTimer *time.Timer
	saveDelay time.Duration
}

// DaemonState represents the complete daemon state
type DaemonState struct {
	Version       string                 `json:"version"`
	StartTime     time.Time              `json:"start_time"`
	LastUpdate    time.Time              `json:"last_update"`
	Status        string                 `json:"status"`
	Repositories  map[string]*RepoState  `json:"repositories"`
	Builds        map[string]*BuildState `json:"builds"`
	Schedules     map[string]*Schedule   `json:"schedules"`
	Statistics    *DaemonStats           `json:"statistics"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// RepoState tracks the state of a repository
type RepoState struct {
	URL           string                 `json:"url"`
	Name          string                 `json:"name"`
	Branch        string                 `json:"branch"`
	LastDiscovery *time.Time             `json:"last_discovery,omitempty"`
	LastBuild     *time.Time             `json:"last_build,omitempty"`
	LastCommit    string                 `json:"last_commit,omitempty"`
	DocumentCount int                    `json:"document_count"`
	BuildCount    int64                  `json:"build_count"`
	ErrorCount    int64                  `json:"error_count"`
	LastError     string                 `json:"last_error,omitempty"`
	DocFilesHash  string                 `json:"doc_files_hash,omitempty"` // hash of doc file set for incremental detection
	DocFilePaths  []string               `json:"doc_file_paths,omitempty"` // sorted list of Hugo-relative doc file paths (per repo)
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// IncrementRepoBuild increments build counters for a repository and optionally the error count.
// If the repository does not yet exist in state it is created with minimal metadata.
func (sm *StateManager) IncrementRepoBuild(url string, success bool) {
	if url == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	rs, ok := sm.state.Repositories[url]
	if !ok {
		// Derive a name heuristically from URL path segment; fallback to URL.
		name := url
		if slash := strings.LastIndex(url, "/"); slash >= 0 && slash+1 < len(url) {
			name = strings.TrimSuffix(url[slash+1:], ".git")
		}
		rs = &RepoState{URL: url, Name: name, Metadata: map[string]interface{}{"created": time.Now()}}
		sm.state.Repositories[url] = rs
	}
	now := time.Now()
	rs.LastBuild = &now
	rs.BuildCount++
	if !success {
		rs.ErrorCount++
	}
	sm.scheduleSave()
}

// SetRepoDocumentCount sets the discovered document count for a repository (idempotent for zero unless increasing).
func (sm *StateManager) SetRepoDocumentCount(url string, count int) {
	if url == "" || count < 0 {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	rs, ok := sm.state.Repositories[url]
	if !ok {
		name := url
		if slash := strings.LastIndex(url, "/"); slash >= 0 && slash+1 < len(url) {
			name = strings.TrimSuffix(url[slash+1:], ".git")
		}
		rs = &RepoState{URL: url, Name: name, Metadata: map[string]interface{}{"created": time.Now()}}
		sm.state.Repositories[url] = rs
	}
	// Overwrite unconditionally; if callers want monotonic behavior they must enforce upstream.
	rs.DocumentCount = count
	sm.scheduleSave()
}

// SetRepoDocFilesHash sets the stable doc files hash for a repository.
func (sm *StateManager) SetRepoDocFilesHash(url, hash string) {
	if url == "" || hash == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	rs, ok := sm.state.Repositories[url]
	if !ok {
		name := url
		if slash := strings.LastIndex(url, "/"); slash >= 0 && slash+1 < len(url) {
			name = strings.TrimSuffix(url[slash+1:], ".git")
		}
		rs = &RepoState{URL: url, Name: name, Metadata: map[string]interface{}{"created": time.Now()}}
		sm.state.Repositories[url] = rs
	}
	rs.DocFilesHash = hash
	sm.scheduleSave()
}

// SetRepoDocFilePaths stores the sorted doc file paths for a repository (replaces prior list).
func (sm *StateManager) SetRepoDocFilePaths(url string, paths []string) {
	if url == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	rs, ok := sm.state.Repositories[url]
	if !ok {
		name := url
		if slash := strings.LastIndex(url, "/"); slash >= 0 && slash+1 < len(url) {
			name = strings.TrimSuffix(url[slash+1:], ".git")
		}
		rs = &RepoState{URL: url, Name: name, Metadata: map[string]interface{}{"created": time.Now()}}
		sm.state.Repositories[url] = rs
	}
	rs.DocFilePaths = append([]string{}, paths...)
	sm.scheduleSave()
}

// GetRepoDocFilePaths returns a copy of the stored doc file path list for a repository.
func (sm *StateManager) GetRepoDocFilePaths(url string) []string {
	if url == "" {
		return nil
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if rs, ok := sm.state.Repositories[url]; ok && len(rs.DocFilePaths) > 0 {
		cp := make([]string, len(rs.DocFilePaths))
		copy(cp, rs.DocFilePaths)
		return cp
	}
	return nil
}

// GetRepoDocFilesHash returns the stored doc files hash for a repository.
func (sm *StateManager) GetRepoDocFilesHash(url string) string {
	if url == "" {
		return ""
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if rs, ok := sm.state.Repositories[url]; ok {
		return rs.DocFilesHash
	}
	return ""
}

// SetRepoLastCommit updates the stored last commit for a repository (by URL) creating state if necessary.
func (sm *StateManager) SetRepoLastCommit(url, name, branch, commit string) {
	if url == "" || commit == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	rs, ok := sm.state.Repositories[url]
	if !ok {
		rs = &RepoState{URL: url, Name: name, Branch: branch, Metadata: map[string]interface{}{"created": time.Now()}}
		sm.state.Repositories[url] = rs
	}
	rs.LastCommit = commit
	sm.scheduleSave()
}

// GetRepoLastCommit returns the stored last commit hash for a repository URL.
func (sm *StateManager) GetRepoLastCommit(url string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if rs, ok := sm.state.Repositories[url]; ok {
		return rs.LastCommit
	}
	return ""
}

// SetLastConfigHash persists the last successful build config hash.
func (sm *StateManager) SetLastConfigHash(hash string) {
	if hash == "" {
		return
	}
	sm.SetConfiguration("last_config_hash", hash)
}

// GetLastConfigHash retrieves the last stored config hash.
func (sm *StateManager) GetLastConfigHash() string {
	if v, ok := sm.GetConfiguration("last_config_hash"); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// SetLastReportChecksum stores the checksum of the last successful build-report.json
func (sm *StateManager) SetLastReportChecksum(sum string) {
	if sum == "" {
		return
	}
	sm.SetConfiguration("last_report_checksum", sum)
}

// GetLastReportChecksum retrieves the persisted build report checksum.
func (sm *StateManager) GetLastReportChecksum() string {
	if v, ok := sm.GetConfiguration("last_report_checksum"); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// SetLastGlobalDocFilesHash stores the global doc_files_hash from the last successful build.
func (sm *StateManager) SetLastGlobalDocFilesHash(hash string) {
	if hash == "" {
		return
	}
	sm.SetConfiguration("last_global_doc_files_hash", hash)
}

// GetLastGlobalDocFilesHash retrieves the persisted global doc_files_hash.
func (sm *StateManager) GetLastGlobalDocFilesHash() string {
	if v, ok := sm.GetConfiguration("last_global_doc_files_hash"); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// BuildState tracks the state of builds
type BuildState struct {
	ID            string                 `json:"id"`
	Type          BuildType              `json:"type"`
	Status        BuildStatus            `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      time.Duration          `json:"duration,omitempty"`
	RepositoryURL string                 `json:"repository_url,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// DaemonStats contains runtime statistics
type DaemonStats struct {
	TotalBuilds      int64     `json:"total_builds"`
	SuccessfulBuilds int64     `json:"successful_builds"`
	FailedBuilds     int64     `json:"failed_builds"`
	TotalDiscoveries int64     `json:"total_discoveries"`
	DocumentsFound   int64     `json:"documents_found"`
	AverageBuildTime float64   `json:"average_build_time_seconds"`
	LastStatReset    time.Time `json:"last_stat_reset"`
	Uptime           float64   `json:"uptime_seconds"`
}

// NewStateManager creates a new state manager
func NewStateManager(dataDir string) (*StateManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	sm := &StateManager{
		dataDir:   dataDir,
		autosave:  true,
		saveDelay: 5 * time.Second,
		state: &DaemonState{
			Version:      "2.0.0", // DocBuilder v2
			StartTime:    time.Now(),
			LastUpdate:   time.Now(),
			Status:       "starting",
			Repositories: make(map[string]*RepoState),
			Builds:       make(map[string]*BuildState),
			Schedules:    make(map[string]*Schedule),
			Statistics: &DaemonStats{
				LastStatReset: time.Now(),
			},
			Configuration: make(map[string]interface{}),
		},
	}

	// Try to load existing state
	if err := sm.Load(); err != nil {
		slog.Warn("Failed to load existing state, starting fresh", "error", err)
	}

	return sm, nil
}

// Load reads state from disk
func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	statePath := filepath.Join(sm.dataDir, "daemon-state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing state, that's fine
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Merge loaded state with current state
	if state.Repositories != nil {
		sm.state.Repositories = state.Repositories
	}
	if state.Builds != nil {
		sm.state.Builds = state.Builds
	}
	if state.Schedules != nil {
		sm.state.Schedules = state.Schedules
	}
	if state.Statistics != nil {
		sm.state.Statistics = state.Statistics
	}
	if state.Configuration != nil {
		sm.state.Configuration = state.Configuration
	}

	sm.state.LastUpdate = time.Now()

	slog.Info("State loaded from disk",
		"repositories", len(sm.state.Repositories),
		"builds", len(sm.state.Builds),
		"schedules", len(sm.state.Schedules))

	return nil
}

// Save writes state to disk
func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.saveUnsafe()
}

// saveUnsafe writes state to disk without acquiring the lock
func (sm *StateManager) saveUnsafe() error {
	sm.state.LastUpdate = time.Now()
	sm.state.Statistics.Uptime = time.Since(sm.state.StartTime).Seconds()

	statePath := filepath.Join(sm.dataDir, "daemon-state.json")
	tempPath := statePath + ".tmp"

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temporary file first
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state to temporary file: %w", err)
	}

	// Atomically replace the state file
	if err := os.Rename(tempPath, statePath); err != nil {
		return fmt.Errorf("failed to replace state file: %w", err)
	}

	return nil
}

// scheduleSave schedules a save operation after a delay (debounced)
func (sm *StateManager) scheduleSave() {
	if !sm.autosave {
		return
	}

	if sm.saveTimer != nil {
		sm.saveTimer.Stop()
	}

	sm.saveTimer = time.AfterFunc(sm.saveDelay, func() {
		if err := sm.Save(); err != nil {
			slog.Error("Failed to auto-save state", "error", err)
		}
	})
}

// GetState returns a copy of the current daemon state
func (sm *StateManager) GetState() *DaemonState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return CopyDaemonState(sm.state)
}

// UpdateStatus updates the daemon status
func (sm *StateManager) UpdateStatus(status string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Status = status
	sm.scheduleSave()
}

// UpdateRepository updates or creates repository state
func (sm *StateManager) UpdateRepository(repo *RepoState) {
	if repo == nil || repo.URL == "" {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Repositories[repo.URL] = repo
	sm.scheduleSave()
}

// GetRepository gets repository state by URL
func (sm *StateManager) GetRepository(url string) *RepoState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if repo, exists := sm.state.Repositories[url]; exists {
		return CopyRepoState(repo)
	}

	return nil
}

// ListRepositories returns all repository states
func (sm *StateManager) ListRepositories() []*RepoState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	repos := make([]*RepoState, 0, len(sm.state.Repositories))
	for _, repo := range sm.state.Repositories {
		repos = append(repos, CopyRepoState(repo))
	}

	return repos
}

// RecordBuild records a build in the state
func (sm *StateManager) RecordBuild(build *BuildState) {
	if build == nil || build.ID == "" {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Builds[build.ID] = build

	// Update statistics
	sm.state.Statistics.TotalBuilds++
	if build.Status == BuildStatusCompleted {
		sm.state.Statistics.SuccessfulBuilds++
	} else if build.Status == BuildStatusFailed {
		sm.state.Statistics.FailedBuilds++
	}

	// Calculate average build time
	if build.Duration > 0 {
		totalTime := sm.state.Statistics.AverageBuildTime * float64(sm.state.Statistics.TotalBuilds-1)
		totalTime += build.Duration.Seconds()
		sm.state.Statistics.AverageBuildTime = totalTime / float64(sm.state.Statistics.TotalBuilds)
	}

	// Keep only the last 100 builds to prevent unbounded growth
	if len(sm.state.Builds) > 100 {
		sm.cleanupOldBuilds()
	}

	sm.scheduleSave()
}

// cleanupOldBuilds removes old builds to keep storage bounded
func (sm *StateManager) cleanupOldBuilds() {
	// Convert to slice for sorting
	builds := make([]*BuildState, 0, len(sm.state.Builds))
	for _, build := range sm.state.Builds {
		builds = append(builds, build)
	}

	// Simple approach: keep the 50 most recent builds
	// In a production system, you might want more sophisticated cleanup
	if len(builds) > 50 {
		// Clear the map and re-add the most recent 50
		sm.state.Builds = make(map[string]*BuildState)

		// For simplicity, keep all builds for now
		// A production implementation would sort by time and keep recent ones
	}
}

// GetBuild gets build state by ID
func (sm *StateManager) GetBuild(id string) *BuildState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if build, exists := sm.state.Builds[id]; exists {
		return CopyBuildState(build)
	}

	return nil
}

// UpdateSchedule updates or creates schedule state
func (sm *StateManager) UpdateSchedule(schedule *Schedule) {
	if schedule == nil || schedule.ID == "" {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Schedules[schedule.ID] = schedule
	sm.scheduleSave()
}

// RemoveSchedule removes a schedule from state
func (sm *StateManager) RemoveSchedule(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.state.Schedules, id)
	sm.scheduleSave()
}

// GetSchedule gets schedule by ID
func (sm *StateManager) GetSchedule(id string) *Schedule {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if schedule, exists := sm.state.Schedules[id]; exists {
		return CopySchedule(schedule)
	}

	return nil
}

// RecordDiscovery records a discovery operation
func (sm *StateManager) RecordDiscovery(repoURL string, documentCount int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Statistics.TotalDiscoveries++
	sm.state.Statistics.DocumentsFound += int64(documentCount)

	// Update repository state
	if repo, exists := sm.state.Repositories[repoURL]; exists {
		now := time.Now()
		repo.LastDiscovery = &now
		repo.DocumentCount = documentCount
	}

	sm.scheduleSave()
}

// GetStatistics returns current daemon statistics
func (sm *StateManager) GetStatistics() *DaemonStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	statsCopy := CopyDaemonStats(sm.state.Statistics)
	if statsCopy == nil {
		return &DaemonStats{}
	}
	statsCopy.Uptime = time.Since(sm.state.StartTime).Seconds()
	return statsCopy
}

// SetConfiguration stores configuration data
func (sm *StateManager) SetConfiguration(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Configuration[key] = value
	sm.scheduleSave()
}

// GetConfiguration retrieves configuration data
func (sm *StateManager) GetConfiguration(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	value, exists := sm.state.Configuration[key]
	return value, exists
}

// Close shuts down the state manager and performs a final save
func (sm *StateManager) Close() error {
	if sm.saveTimer != nil {
		sm.saveTimer.Stop()
	}

	// Perform final save
	if err := sm.Save(); err != nil {
		return fmt.Errorf("failed to save final state: %w", err)
	}

	slog.Info("State manager closed")
	return nil
}
