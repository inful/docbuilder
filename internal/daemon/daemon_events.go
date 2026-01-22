package daemon

import (
	"context"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// GetBuildProjection returns the build history projection for querying build history.
// Returns nil if event sourcing is not initialized.
func (d *Daemon) GetBuildProjection() *eventstore.BuildHistoryProjection {
	return d.buildProjection
}

// EmitBuildEvent persists an event to the event store and updates the projection.
// This delegates to the eventEmitter component.
func (d *Daemon) EmitBuildEvent(ctx context.Context, event eventstore.Event) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitEvent(ctx, event)
}

// EmitBuildStarted implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildStarted(ctx context.Context, buildID string, meta eventstore.BuildStartedMeta) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildStarted(ctx, buildID, meta)
}

// EmitBuildCompleted implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildCompleted(ctx context.Context, buildID string, duration time.Duration, artifacts map[string]string) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildCompleted(ctx, buildID, duration, artifacts)
}

// EmitBuildFailed implements BuildEventEmitter for the daemon.
func (d *Daemon) EmitBuildFailed(ctx context.Context, buildID, stage, errorMsg string) error {
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildFailed(ctx, buildID, stage, errorMsg)
}

// onBuildReportEmitted is called after a build report is emitted to the event store.
// This is where we trigger post-build hooks like link verification and state updates.
func (d *Daemon) onBuildReportEmitted(ctx context.Context, buildID string, report *models.BuildReport) error {
	// Update state manager after successful builds.
	// This is critical for skip evaluation to work correctly on subsequent builds.
	if report != nil && report.Outcome == models.OutcomeSuccess && d.stateManager != nil && d.config != nil {
		d.updateStateAfterBuild(report)
	}

	// Trigger link verification after successful builds (low priority background task).
	slog.Debug("onBuildReportEmitted called",
		"build_id", buildID,
		"report_nil", report == nil,
		"outcome", func() string {
			if report != nil {
				return string(report.Outcome)
			}
			return "N/A"
		}(),
		"verifier_nil", d.linkVerifier == nil)
	if report != nil && report.Outcome == models.OutcomeSuccess && d.linkVerifier != nil {
		go d.verifyLinksAfterBuild(ctx, buildID)
	}

	return nil
}

// EmitBuildReport implements BuildEventEmitter for the daemon (legacy/compatibility).
// This is now handled by EventEmitter calling onBuildReportEmitted.
func (d *Daemon) EmitBuildReport(ctx context.Context, buildID string, report *models.BuildReport) error {
	// Delegate to event emitter which will call back to onBuildReportEmitted.
	if d.eventEmitter == nil {
		return nil
	}
	return d.eventEmitter.EmitBuildReport(ctx, buildID, report)
}
