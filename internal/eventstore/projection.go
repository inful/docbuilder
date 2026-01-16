// Package eventstore provides event sourcing primitives for build tracking.
package eventstore

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const (
	buildStatusRunning   = "running"
	buildStatusCompleted = "completed"
)

// BuildSummary is a read model summarizing a completed or in-progress build.
type BuildSummary struct {
	BuildID      string            `json:"build_id"`
	TenantID     string            `json:"tenant_id,omitempty"`
	Status       string            `json:"status"` // "running", "completed", "failed"
	StartedAt    time.Time         `json:"started_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	Duration     time.Duration     `json:"duration,omitempty"`
	RepoCount    int               `json:"repo_count"`
	FileCount    int               `json:"file_count"`
	ErrorStage   string            `json:"error_stage,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Artifacts    map[string]string `json:"artifacts,omitempty"`
	// ReportData contains detailed build report metrics (populated from BuildReportGenerated event)
	ReportData *BuildReportData `json:"report_data,omitempty"`
}

// BuildHistoryProjection maintains an in-memory view of build history,
// reconstructed from events stored in the event store.
type BuildHistoryProjection struct {
	mu       sync.RWMutex
	store    Store
	builds   map[string]*BuildSummary // buildID -> summary
	history  []*BuildSummary          // ordered by start time, newest first
	maxSize  int
	lastSync time.Time
}

// NewBuildHistoryProjection creates a new projection backed by the given store.
func NewBuildHistoryProjection(store Store, maxHistorySize int) *BuildHistoryProjection {
	if maxHistorySize <= 0 {
		maxHistorySize = 100
	}
	return &BuildHistoryProjection{
		store:   store,
		builds:  make(map[string]*BuildSummary),
		history: make([]*BuildSummary, 0, maxHistorySize),
		maxSize: maxHistorySize,
	}
}

// Rebuild reconstructs the projection from all events in the store.
// This is typically called at startup.
func (p *BuildHistoryProjection) Rebuild(ctx context.Context) error {
	// Get all events from the beginning of time
	events, err := p.store.GetRange(ctx, time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Reset state
	p.builds = make(map[string]*BuildSummary)
	p.history = make([]*BuildSummary, 0, p.maxSize)

	// Apply each event
	for _, event := range events {
		p.applyEventLocked(event)
	}

	// Sort history by start time (newest first)
	p.sortHistoryLocked()

	// Trim to max size
	if len(p.history) > p.maxSize {
		p.history = p.history[:p.maxSize]
	}

	// Prevent unbounded growth: keep only bounded history + any running builds.
	p.pruneBuildsLocked()

	p.lastSync = time.Now()
	return nil
}

// Apply processes a single event and updates the projection.
// This is used for real-time updates when events are emitted.
func (p *BuildHistoryProjection) Apply(event Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.applyEventLocked(event)
}

// applyEventLocked applies an event without holding the lock.
func (p *BuildHistoryProjection) applyEventLocked(event Event) {
	buildID := event.BuildID()
	if buildID == "" || buildID == "unknown" {
		return
	}

	summary, exists := p.builds[buildID]
	if !exists {
		summary = &BuildSummary{
			BuildID:   buildID,
			Status:    "running",
			StartedAt: event.Timestamp(),
		}
		p.builds[buildID] = summary
	}

	// Update summary based on event type
	switch event.Type() {
	case "BuildStarted":
		summary.StartedAt = event.Timestamp()
		summary.Status = buildStatusRunning
		// Parse payload for tenant_id
		var payload struct {
			TenantID string `json:"tenant_id"`
		}
		if err := json.Unmarshal(event.Payload(), &payload); err == nil {
			summary.TenantID = payload.TenantID
		}

	case "RepositoryCloned":
		summary.RepoCount++

	case "DocumentsDiscovered":
		var payload struct {
			FileCount int `json:"file_count"`
		}
		if err := json.Unmarshal(event.Payload(), &payload); err == nil {
			summary.FileCount += payload.FileCount
		}

	case "BuildCompleted":
		now := event.Timestamp()
		summary.CompletedAt = &now
		summary.Duration = now.Sub(summary.StartedAt)
		summary.Status = buildStatusCompleted
		var payload struct {
			Status    string            `json:"status"`
			Artifacts map[string]string `json:"artifacts"`
		}
		if err := json.Unmarshal(event.Payload(), &payload); err == nil {
			if payload.Status != "" {
				summary.Status = payload.Status
			}
			summary.Artifacts = payload.Artifacts
		}
		// Add to history if not already there
		p.addToHistoryLocked(summary)

	case "BuildFailed":
		now := event.Timestamp()
		summary.CompletedAt = &now
		summary.Duration = now.Sub(summary.StartedAt)
		summary.Status = "failed"
		var payload struct {
			Stage string `json:"stage"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(event.Payload(), &payload); err == nil {
			summary.ErrorStage = payload.Stage
			summary.ErrorMessage = payload.Error
		}
		// Add to history if not already there
		p.addToHistoryLocked(summary)

	case "BuildReportGenerated":
		var report BuildReportData
		if err := json.Unmarshal(event.Payload(), &report); err == nil {
			summary.ReportData = &report
		}
	}
}

// addToHistoryLocked adds a completed build to history if not already present.
func (p *BuildHistoryProjection) addToHistoryLocked(summary *BuildSummary) {
	// Check if already in history
	for _, h := range p.history {
		if h.BuildID == summary.BuildID {
			return
		}
	}

	// Add to history
	p.history = append([]*BuildSummary{summary}, p.history...)

	// Trim to max size
	if len(p.history) > p.maxSize {
		p.history = p.history[:p.maxSize]
	}

	// Prevent unbounded growth: keep only bounded history + any running builds.
	p.pruneBuildsLocked()
}

// pruneBuildsLocked removes completed builds not present in the bounded history.
// It keeps any builds that are still marked as running.
// Caller must hold p.mu (write lock).
func (p *BuildHistoryProjection) pruneBuildsLocked() {
	keep := make(map[string]struct{}, len(p.history))
	for _, h := range p.history {
		if h != nil {
			keep[h.BuildID] = struct{}{}
		}
	}

	for id, summary := range p.builds {
		if summary != nil && summary.Status == buildStatusRunning {
			continue
		}
		if _, ok := keep[id]; !ok {
			delete(p.builds, id)
		}
	}
}

// sortHistoryLocked sorts history by start time, newest first.
func (p *BuildHistoryProjection) sortHistoryLocked() {
	// Simple insertion sort (history is usually small)
	for i := 1; i < len(p.history); i++ {
		for j := i; j > 0 && p.history[j].StartedAt.After(p.history[j-1].StartedAt); j-- {
			p.history[j], p.history[j-1] = p.history[j-1], p.history[j]
		}
	}
}

// GetHistory returns the build history, newest first.
func (p *BuildHistoryProjection) GetHistory() []*BuildSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*BuildSummary, len(p.history))
	copy(result, p.history)
	return result
}

// GetBuild returns the summary for a specific build.
func (p *BuildHistoryProjection) GetBuild(buildID string) (*BuildSummary, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	summary, exists := p.builds[buildID]
	if !exists {
		return nil, false
	}

	// Return a copy
	cp := *summary
	return &cp, true
}

// GetActiveBuild returns a currently running build if any.
func (p *BuildHistoryProjection) GetActiveBuild() *BuildSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, summary := range p.builds {
		if summary.Status == "running" {
			cp := *summary
			return &cp
		}
	}
	return nil
}

// GetLastCompletedBuild returns the most recently completed build (success or failure).
func (p *BuildHistoryProjection) GetLastCompletedBuild() *BuildSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.history) == 0 {
		return nil
	}

	// History is sorted newest first
	cp := *p.history[0]
	return &cp
}

// LastSyncTime returns when the projection was last synchronized.
func (p *BuildHistoryProjection) LastSyncTime() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastSync
}
