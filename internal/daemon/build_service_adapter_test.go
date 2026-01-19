package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// mockBuildService is a test double for build.BuildService.
type mockBuildService struct {
	runFunc func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error)
}

func (m *mockBuildService) Run(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, req)
	}
	return &build.BuildResult{
		Status:         build.BuildStatusSuccess,
		Repositories:   1,
		FilesProcessed: 10,
		Duration:       time.Second,
		StartTime:      time.Now().Add(-time.Second),
		EndTime:        time.Now(),
	}, nil
}

func TestBuildServiceAdapter_Build(t *testing.T) {
	t.Run("nil job", func(t *testing.T) {
		adapter := NewBuildServiceAdapter(&mockBuildService{})
		report, err := adapter.Build(t.Context(), nil)
		if err == nil {
			t.Error("expected error for nil job")
		}
		if report != nil {
			t.Errorf("expected nil report for nil job")
		}
	})

	t.Run("missing config", func(t *testing.T) {
		adapter := NewBuildServiceAdapter(&mockBuildService{})
		job := &BuildJob{
			ID:        "test",
			TypedMeta: &BuildJobMetadata{},
		}
		report, err := adapter.Build(t.Context(), job)
		if err == nil {
			t.Error("expected error for missing config")
		}
		if report != nil {
			t.Errorf("expected nil report for missing config")
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := &mockBuildService{
			runFunc: func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error) {
				return &build.BuildResult{
					Status:         build.BuildStatusSuccess,
					Repositories:   2,
					FilesProcessed: 15,
					Duration:       500 * time.Millisecond,
					StartTime:      time.Now().Add(-500 * time.Millisecond),
					EndTime:        time.Now(),
					Report: &models.BuildReport{
						Outcome:      models.OutcomeSuccess,
						Repositories: 2,
						Files:        15,
					},
				}, nil
			},
		}

		adapter := NewBuildServiceAdapter(svc)
		job := &BuildJob{
			ID: "test-job",
			TypedMeta: &BuildJobMetadata{
				V2Config: &config.Config{
					Output: config.OutputConfig{
						Directory: "/tmp/test",
					},
				},
			},
		}

		report, err := adapter.Build(t.Context(), job)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report == nil {
			t.Fatal("expected non-nil report")
		}
		if report.Outcome != models.OutcomeSuccess {
			t.Errorf("expected outcome %s, got %s", models.OutcomeSuccess, report.Outcome)
		}
		if report.Repositories != 2 {
			t.Errorf("expected 2 repositories, got %d", report.Repositories)
		}
		if report.Files != 15 {
			t.Errorf("expected 15 files, got %d", report.Files)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		svc := &mockBuildService{
			runFunc: func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error) {
				return &build.BuildResult{
					Status: build.BuildStatusCancelled,
					Report: &models.BuildReport{
						Outcome: models.OutcomeCanceled,
					},
				}, context.Canceled
			},
		}

		adapter := NewBuildServiceAdapter(svc)
		job := &BuildJob{
			ID:        "test-job",
			TypedMeta: &BuildJobMetadata{V2Config: &config.Config{}},
		}

		_, err := adapter.Build(t.Context(), job)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("skipped", func(t *testing.T) {
		svc := &mockBuildService{
			runFunc: func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error) {
				return &build.BuildResult{
					Status:     build.BuildStatusSkipped,
					Skipped:    true,
					SkipReason: "no changes detected",
					Report: &models.BuildReport{
						Outcome:    models.OutcomeSuccess,
						SkipReason: "no changes detected",
					},
				}, nil
			},
		}

		adapter := NewBuildServiceAdapter(svc)
		job := &BuildJob{
			ID:        "test-job",
			TypedMeta: &BuildJobMetadata{V2Config: &config.Config{}},
		}

		report, err := adapter.Build(t.Context(), job)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report == nil {
			t.Fatal("expected non-nil report")
		}
		if report.Outcome != models.OutcomeSuccess {
			t.Errorf("expected outcome %s for skipped, got %s", models.OutcomeSuccess, report.Outcome)
		}
		if report.SkipReason != "no changes detected" {
			t.Errorf("expected skip reason 'no changes detected', got %q", report.SkipReason)
		}
	})
}

func TestBuildServiceAdapter_ImplementsBuilder(t *testing.T) {
	var _ Builder = (*BuildServiceAdapter)(nil)
}
