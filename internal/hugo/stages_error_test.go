package hugo

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// fake stage functions for testing classification.
func failingFatalStage(_ context.Context, _ *BuildState) error {
	return newFatalStageError(StageName("fatal_stage"), errors.New("boom"))
}

func failingWarnStage(_ context.Context, _ *BuildState) error {
	return newWarnStageError(StageName("warn_stage"), errors.New("soft"))
}

func TestRunStages_ErrorClassification(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator(cfg, t.TempDir())
	report := newBuildReport(context.Background(), 0, 0)
	bs := newBuildState(gen, nil, report)

	stages := []StageDef{{StageName("warn_stage"), failingWarnStage}, {StageName("fatal_stage"), failingFatalStage}}

	err := runStages(context.Background(), bs, stages)
	if err == nil {
		t.Fatalf("expected fatal error")
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(report.Warnings))
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 fatal error, got %d", len(report.Errors))
	}
	if report.StageErrorKinds[StageName("warn_stage")] != StageErrorWarning {
		t.Fatalf("expected warning kind recorded")
	}
	if report.StageErrorKinds[StageName("fatal_stage")] != StageErrorFatal {
		t.Fatalf("fatal_stage kind mismatch")
	}
}

func TestRunStages_Canceled(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator(cfg, t.TempDir())
	report := newBuildReport(context.Background(), 0, 0)
	bs := newBuildState(gen, nil, report)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runStages(ctx, bs, []StageDef{{StagePrepareOutput, stagePrepareOutput}})
	if err == nil {
		t.Fatalf("expected canceled error")
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 canceled error recorded, got %d", len(report.Errors))
	}
	if report.StageErrorKinds[StagePrepareOutput] != StageErrorCanceled {
		t.Fatalf("expected canceled kind for prepare_output")
	}
}

func TestRunStages_TimingRecordedOnWarning(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator(cfg, t.TempDir())
	report := newBuildReport(context.Background(), 0, 0)
	bs := newBuildState(gen, nil, report)

	stages := []StageDef{{StageName("warn_stage"), failingWarnStage}}
	if err := runStages(context.Background(), bs, stages); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := report.StageDurations["warn_stage"]; !ok {
		t.Fatalf("expected timing recorded for warn_stage")
	}
	if report.StageErrorKinds[StageName("warn_stage")] != StageErrorWarning {
		t.Fatalf("expected warning kind recorded")
	}
	// Sanity check timing value
	if report.StageDurations["warn_stage"] <= 0 || report.StageDurations["warn_stage"] > 1*time.Second {
		t.Fatalf("unexpected duration range: %v", report.StageDurations["warn_stage"])
	}
}
