package daemon

import (
	"context"
	"fmt"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// EventEmitter handles build lifecycle event emission to the event store.
// It encapsulates the logic for persisting events and updating projections.
type EventEmitter struct {
	store      eventstore.Store
	projection *eventstore.BuildHistoryProjection
	daemon     *Daemon // Reference back to daemon for hooks like link verification
}

// NewEventEmitter creates a new EventEmitter with the given store and projection.
func NewEventEmitter(store eventstore.Store, projection *eventstore.BuildHistoryProjection) *EventEmitter {
	return &EventEmitter{
		store:      store,
		projection: projection,
	}
}

// EmitEvent persists an event to the event store and updates the projection.
// This is the canonical way to record build lifecycle events.
func (e *EventEmitter) EmitEvent(ctx context.Context, event eventstore.Event) error {
	if e.store == nil {
		return nil // Event store not initialized
	}

	// Persist to store
	if err := e.store.Append(ctx, event.BuildID(), event.Type(), event.Payload(), event.Metadata()); err != nil {
		return fmt.Errorf("failed to persist event: %w", err)
	}

	// Update projection
	if e.projection != nil {
		e.projection.Apply(event)
	}

	return nil
}

// EmitBuildStarted implements BuildEventEmitter.
func (e *EventEmitter) EmitBuildStarted(ctx context.Context, buildID string, meta eventstore.BuildStartedMeta) error {
	event, err := eventstore.NewBuildStarted(buildID, meta)
	if err != nil {
		return err
	}
	return e.EmitEvent(ctx, event)
}

// EmitBuildCompleted implements BuildEventEmitter.
func (e *EventEmitter) EmitBuildCompleted(ctx context.Context, buildID string, duration time.Duration, artifacts map[string]string) error {
	event, err := eventstore.NewBuildCompleted(buildID, "completed", duration, artifacts)
	if err != nil {
		return err
	}
	return e.EmitEvent(ctx, event)
}

// EmitBuildFailed implements BuildEventEmitter.
func (e *EventEmitter) EmitBuildFailed(ctx context.Context, buildID, stage, errorMsg string) error {
	event, err := eventstore.NewBuildFailed(buildID, stage, errorMsg)
	if err != nil {
		return err
	}
	return e.EmitEvent(ctx, event)
}

// EmitBuildReport emits a build report event with the given report data.
func (e *EventEmitter) EmitBuildReport(ctx context.Context, buildID string, report *hugo.BuildReport) error {
	if report == nil {
		return nil
	}

	reportData := convertBuildReportToEventData(report)

	event, err := eventstore.NewBuildReportGenerated(buildID, reportData)
	if err != nil {
		return err
	}
	if err := e.EmitEvent(ctx, event); err != nil {
		return err
	}

	// Trigger daemon hooks (like link verification) after event is persisted
	if e.daemon != nil {
		return e.daemon.onBuildReportEmitted(ctx, buildID, report)
	}

	return nil
}

// convertBuildReportToEventData converts a hugo.BuildReport to eventstore.BuildReportData.
func convertBuildReportToEventData(report *hugo.BuildReport) eventstore.BuildReportData {
	reportData := eventstore.BuildReportData{
		Outcome:             string(report.Outcome),
		Summary:             report.Summary(),
		RenderedPages:       report.RenderedPages,
		ClonedRepositories:  report.ClonedRepositories,
		FailedRepositories:  report.FailedRepositories,
		SkippedRepositories: report.SkippedRepositories,
		StaticRendered:      report.StaticRendered,
	}

	// Convert stage durations
	if len(report.StageDurations) > 0 {
		reportData.StageDurations = make(map[string]int64, len(report.StageDurations))
		for k, v := range report.StageDurations {
			reportData.StageDurations[k] = v.Milliseconds()
		}
	}

	// Convert errors (truncate long messages)
	for _, e := range report.Errors {
		msg := e.Error()
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		reportData.Errors = append(reportData.Errors, msg)
	}

	// Convert warnings (truncate long messages)
	for _, w := range report.Warnings {
		msg := w.Error()
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		reportData.Warnings = append(reportData.Warnings, msg)
	}

	return reportData
}

// Compile-time check that EventEmitter implements BuildEventEmitter.
var _ BuildEventEmitter = (*EventEmitter)(nil)
