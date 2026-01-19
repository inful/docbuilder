package hugo

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// fake stage functions for testing classification.
func failingFatalStage(_ context.Context, _ *models.BuildState) error {
	return models.NewFatalStageError(models.StageName("fatal_stage"), errors.New("boom"))
}

func failingWarnStage(_ context.Context, _ *models.BuildState) error {
	return models.NewWarnStageError(models.StageName("warn_stage"), errors.New("soft"))
}

func TestRunStages_ErrorClassification(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator(cfg, t.TempDir())
	report := models.NewBuildReport(t.Context(), 0, 0)
	bs := models.NewBuildState(gen, nil, report)

	stageDefs := []models.StageDef{
		{Name: models.StageName("warn_stage"), Fn: failingWarnStage},
		{Name: models.StageName("fatal_stage"), Fn: failingFatalStage},
	}

	err := stages.RunStages(t.Context(), bs, stageDefs)
	if err == nil {
		t.Fatalf("expected fatal error")
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(report.Warnings))
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 fatal error, got %d", len(report.Errors))
	}
	if report.StageErrorKinds[models.StageName("warn_stage")] != models.StageErrorWarning {
		t.Fatalf("expected warning kind recorded")
	}
	if report.StageErrorKinds[models.StageName("fatal_stage")] != models.StageErrorFatal {
		t.Fatalf("fatal_stage kind mismatch")
	}
}

func TestRunStages_Canceled(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator(cfg, t.TempDir())
	report := models.NewBuildReport(t.Context(), 0, 0)
	bs := models.NewBuildState(gen, nil, report)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := stages.RunStages(ctx, bs, []models.StageDef{{Name: models.StagePrepareOutput, Fn: stages.StagePrepareOutput}})
	if err == nil {
		t.Fatalf("expected canceled error")
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 canceled error recorded, got %d", len(report.Errors))
	}
	if report.StageErrorKinds[models.StagePrepareOutput] != models.StageErrorCanceled {
		t.Fatalf("expected canceled kind for prepare_output")
	}
}

func TestRunStages_TimingRecordedOnWarning(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator(cfg, t.TempDir())
	report := models.NewBuildReport(t.Context(), 0, 0)
	bs := models.NewBuildState(gen, nil, report)

	stageDefs := []models.StageDef{{Name: models.StageName("warn_stage"), Fn: failingWarnStage}}
	if err := stages.RunStages(t.Context(), bs, stageDefs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := report.StageDurations["warn_stage"]; !ok {
		t.Fatalf("expected timing recorded for warn_stage")
	}
	if report.StageErrorKinds[models.StageName("warn_stage")] != models.StageErrorWarning {
		t.Fatalf("expected warning kind recorded")
	}
	// Sanity check timing value
	if report.StageDurations["warn_stage"] <= 0 || report.StageDurations["warn_stage"] > 1*time.Second {
		t.Fatalf("unexpected duration range: %v", report.StageDurations["warn_stage"])
	}
}
