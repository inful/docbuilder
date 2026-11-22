// Package eventstore provides event sourcing primitives for build tracking.
package eventstore

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
	mu      sync.RWMutex
	store   Store
	builds  map[string]*BuildSummary // buildID -> summary
	history []*BuildSummary          // ordered by start time, newest first
	maxSize int
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

// maxRebuildEvents bounds how many events a rebuild replays. The
// projection keeps the last maxSize builds, so the events of older builds
// are not needed.
const maxRebuildEvents = 2000

// Rebuild reconstructs the projection from the stored events.
// This is typically called at startup.
func (p *BuildHistoryProjection) Rebuild(ctx context.Context) error {
	// One event more than the limit tells us whether the window cut the
	// log short.
	events, err := p.store.GetRecent(ctx, maxRebuildEvents+1)
	if err != nil {
		return errors.WrapError(err, errors.CategoryEventStore, "failed to retrieve events for reconstruction").
			WithCause(ErrProjectionRebuildFailed).
			Build()
	}

	// The window can cut a build in half. Its first events alone would
	// make it look like it is still running, so that build is dropped.
	if len(events) > maxRebuildEvents {
		oldestBuild := events[0].BuildID()
		kept := events[:0]
		for _, event := range events {
			if event.BuildID() == oldestBuild {
				continue
			}
			kept = append(kept, event)
		}
		events = kept
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
			Status:    buildStatusRunning,
			StartedAt: event.Timestamp(),
		}
		p.builds[buildID] = summary
	}

	// Update summary based on event type
	switch event.Type() {
	case eventTypeBuildStarted:
		summary.StartedAt = event.Timestamp()
		summary.Status = buildStatusRunning
		// Parse payload for tenant_id
		var payload struct {
			TenantID string `json:"tenant_id"`
		}
		if err := json.Unmarshal(event.Payload(), &payload); err == nil {
			summary.TenantID = payload.TenantID
		}

	case eventTypeRepositoryCloned:
		summary.RepoCount++

	case eventTypeDocumentsDiscovered:
		var payload struct {
			FileCount int `json:"file_count"`
		}
		if err := json.Unmarshal(event.Payload(), &payload); err == nil {
			summary.FileCount += payload.FileCount
		}

	case eventTypeBuildCompleted:
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

	case eventTypeBuildFailed:
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
		// The summary map holds every build ever seen; drop the ones that
		// just fell out of the history so it cannot grow without bound.
		kept := make(map[string]struct{}, len(p.history))
		for _, h := range p.history {
			kept[h.BuildID] = struct{}{}
		}
		for id := range p.builds {
			if _, ok := kept[id]; !ok {
				delete(p.builds, id)
			}
		}
	}
}

// sortHistoryLocked sorts history by start time, newest first.
func (p *BuildHistoryProjection) sortHistoryLocked() {
	slices.SortFunc(p.history, func(a, b *BuildSummary) int {
		return b.StartedAt.Compare(a.StartedAt)
	})
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
