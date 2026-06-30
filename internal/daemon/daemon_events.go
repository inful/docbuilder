package daemon

import (
	"context"
	"log/slog"

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

// onBuildReportEmitted is called after a build report is emitted to the event store.
// This is where we trigger post-build hooks like link verification and state updates.
func (d *Daemon) onBuildReportEmitted(ctx context.Context, buildID string, report *models.BuildReport) error {
	// Decide whether to run link verification before updating state so the decision
	// can be based on what actually happened in this build.
	shouldVerify := report != nil && report.Outcome == models.OutcomeSuccess && d.linkVerifier != nil && shouldRunLinkVerification(report)
	if report != nil && report.Outcome == models.OutcomeSuccess && d.linkVerifier != nil && !shouldVerify {
		slog.Debug("Skipping post-build link verification",
			"build_id", buildID,
			"skip_reason", report.SkipReason)
	}

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
	if shouldVerify {
		go d.verifyLinksAfterBuild(ctx, buildID)
	}

	return nil
}
