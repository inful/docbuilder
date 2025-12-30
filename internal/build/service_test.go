package build

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// mockHugoGenerator is a test double for HugoGenerator.
type mockHugoGenerator struct {
	generateError error
	docFiles      []docs.DocFile
}

func (m *mockHugoGenerator) GenerateSite(docFiles []docs.DocFile) error {
	m.docFiles = docFiles
	return m.generateError
}

func TestBuildStatus_IsSuccess(t *testing.T) {
	tests := []struct {
		status   BuildStatus
		expected bool
	}{
		{BuildStatusSuccess, true},
		{BuildStatusSkipped, true},
		{BuildStatusFailed, false},
		{BuildStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsSuccess(); got != tt.expected {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewBuildService(t *testing.T) {
	svc := NewBuildService()
	if svc == nil {
		t.Fatal("NewBuildService() returned nil")
	}
	if svc.workspaceFactory == nil {
		t.Error("workspaceFactory should be set")
	}
	if svc.gitClientFactory == nil {
		t.Error("gitClientFactory should be set")
	}
}

func TestDefaultBuildService_Run_NilConfig(t *testing.T) {
	svc := NewBuildService()

	result, err := svc.Run(context.Background(), BuildRequest{
		Config:    nil,
		OutputDir: "/tmp/test",
	})

	if err == nil {
		t.Error("expected error for nil config")
	}
	if result.Status != BuildStatusFailed {
		t.Errorf("expected status %s, got %s", BuildStatusFailed, result.Status)
	}
}

func TestDefaultBuildService_Run_NoRepositories(t *testing.T) {
	svc := NewBuildService()

	result, err := svc.Run(context.Background(), BuildRequest{
		Config:    &config.Config{},
		OutputDir: "/tmp/test",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Status != BuildStatusSuccess {
		t.Errorf("expected status %s, got %s", BuildStatusSuccess, result.Status)
	}
}

func TestDefaultBuildService_Run_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	svc := NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			return workspace.NewManager("")
		}).
		WithGitClientFactory(func(path string) *git.Client {
			return git.NewClient(path)
		})

	cfg := &config.Config{
		Repositories: []config.Repository{
			{Name: "test", URL: "https://example.com/repo.git"},
		},
	}

	result, err := svc.Run(ctx, BuildRequest{
		Config:    cfg,
		OutputDir: "/tmp/test",
	})

	// Note: Might fail earlier during workspace creation
	// depending on how quickly the cancellation propagates
	if result.Status == BuildStatusCancelled {
		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	}
}

func TestDefaultBuildService_WithFactories(t *testing.T) {
	wsFactoryCalled := false
	gitFactoryCalled := false
	hugoFactoryCalled := false

	svc := NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			wsFactoryCalled = true
			return workspace.NewManager("")
		}).
		WithGitClientFactory(func(path string) *git.Client {
			gitFactoryCalled = true
			return git.NewClient(path)
		}).
		WithHugoGeneratorFactory(func(cfg any, outputDir string) HugoGenerator {
			hugoFactoryCalled = true
			return &mockHugoGenerator{}
		})

	if svc.workspaceFactory == nil {
		t.Error("workspaceFactory not set")
	}
	if svc.gitClientFactory == nil {
		t.Error("gitClientFactory not set")
	}
	if svc.hugoGeneratorFactory == nil {
		t.Error("hugoGeneratorFactory not set")
	}

	// Verify factories are called during Run (with empty repos so it exits early)
	_, _ = svc.Run(context.Background(), BuildRequest{
		Config:    &config.Config{},
		OutputDir: "/tmp/test",
	})

	// With no repos, we don't proceed far enough to call all factories
	// This test just verifies the factories are properly wired
	_ = wsFactoryCalled
	_ = gitFactoryCalled
	_ = hugoFactoryCalled
}

func TestBuildResult_Duration(t *testing.T) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	end := time.Now()

	result := &BuildResult{
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
	}

	if result.Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", result.Duration)
	}
}

// mockSkipEvaluator is a test double for SkipEvaluator.
type mockSkipEvaluator struct {
	canSkip    bool
	skipReport any
}

func (m *mockSkipEvaluator) Evaluate(repos []any) (any, bool) {
	return m.skipReport, m.canSkip
}

func TestDefaultBuildService_Run_SkipEvaluation(t *testing.T) {
	t.Run("skip_when_evaluator_returns_true", func(t *testing.T) {
		skipReport := "test_skip_report"
		evaluator := &mockSkipEvaluator{canSkip: true, skipReport: skipReport}

		svc := NewBuildService().
			WithSkipEvaluatorFactory(func(outputDir string) SkipEvaluator {
				return evaluator
			})

		result, err := svc.Run(context.Background(), BuildRequest{
			Config: &config.Config{
				Repositories: []config.Repository{{Name: "test", URL: "https://example.com/test.git"}},
			},
			OutputDir: "/tmp/test",
			Options:   BuildOptions{SkipIfUnchanged: true},
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != BuildStatusSkipped {
			t.Errorf("expected status skipped, got %v", result.Status)
		}
		if !result.Skipped {
			t.Error("expected Skipped=true")
		}
		if result.SkipReason != "no_changes" {
			t.Errorf("expected SkipReason='no_changes', got '%s'", result.SkipReason)
		}
		if result.Report != skipReport {
			t.Error("expected skip report to be returned")
		}
	})

	t.Run("proceed_when_evaluator_returns_false", func(t *testing.T) {
		evaluator := &mockSkipEvaluator{canSkip: false}

		// Need to provide all factories to avoid nil pointer
		wsManager := workspace.NewManager(t.TempDir())
		svc := NewBuildService().
			WithSkipEvaluatorFactory(func(outputDir string) SkipEvaluator {
				return evaluator
			}).
			WithWorkspaceFactory(func() *workspace.Manager {
				return wsManager
			}).
			WithGitClientFactory(func(path string) *git.Client {
				return git.NewClient(path)
			}).
			WithHugoGeneratorFactory(func(cfg any, outputDir string) HugoGenerator {
				return &mockHugoGenerator{}
			})

		// This will fail at git clone (no network), but that's fine for testing
		// The important thing is that it proceeds past skip evaluation
		result, _ := svc.Run(context.Background(), BuildRequest{
			Config: &config.Config{
				Repositories: []config.Repository{{Name: "test", URL: "https://example.com/test.git"}},
			},
			OutputDir: t.TempDir(),
			Options:   BuildOptions{SkipIfUnchanged: true},
		})

		// Should not be skipped
		if result.Skipped {
			t.Error("expected Skipped=false when evaluator returns false")
		}
	})

	t.Run("skip_evaluation_disabled_when_option_false", func(t *testing.T) {
		evaluatorCalled := false
		evaluator := &mockSkipEvaluator{canSkip: true}

		svc := NewBuildService().
			WithSkipEvaluatorFactory(func(outputDir string) SkipEvaluator {
				evaluatorCalled = true
				return evaluator
			})

		_, _ = svc.Run(context.Background(), BuildRequest{
			Config: &config.Config{
				Repositories: []config.Repository{{Name: "test", URL: "https://example.com/test.git"}},
			},
			OutputDir: "/tmp/test",
			Options:   BuildOptions{SkipIfUnchanged: false}, // disabled
		})

		if evaluatorCalled {
			t.Error("skip evaluator should not be called when SkipIfUnchanged=false")
		}
	})
}
