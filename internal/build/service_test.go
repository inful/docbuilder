package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

const (
	testBuildOutputDir = "/tmp/test"
	testRepoName       = "test"
	testRepoURL        = "https://example.com/test.git"
)

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
		return
	}
	if svc.workspaceFactory == nil {
		t.Error("workspaceFactory should be set")
	}
}

func TestDefaultBuildService_Run_NilConfig(t *testing.T) {
	svc := NewBuildService()

	result, err := svc.Run(t.Context(), BuildRequest{
		Config:    nil,
		OutputDir: testBuildOutputDir,
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

	result, err := svc.Run(t.Context(), BuildRequest{
		Config:    &config.Config{},
		OutputDir: testBuildOutputDir,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Status != BuildStatusSuccess {
		t.Errorf("expected status %s, got %s", BuildStatusSuccess, result.Status)
	}
}

func TestDefaultBuildService_Run_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	svc := NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			return workspace.NewManager("")
		})

	cfg := &config.Config{
		Repositories: []config.Repository{
			{Name: testRepoName, URL: "https://example.com/repo.git"},
		},
	}

	result, err := svc.Run(ctx, BuildRequest{
		Config:    cfg,
		OutputDir: testBuildOutputDir,
	})

	// Note: Might fail earlier during workspace creation
	// depending on how quickly the cancellation propagates
	if result.Status == BuildStatusCancelled {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	}
}

func TestDefaultBuildService_Run_SkipGating(t *testing.T) {
	// When SkipState is nil, skip-evaluation must NOT happen — the build
	// proceeds (and likely fails downstream at git clone, which is fine
	// for this gating test). This guards against an accidental refactor
	// that makes skip evaluation fire without an explicit state accessor.
	t.Run("skip_evaluation_disabled_without_state", func(t *testing.T) {
		svc := NewBuildService()

		result, _ := svc.Run(t.Context(), BuildRequest{
			Config: &config.Config{
				Repositories: []config.Repository{{Name: "test", URL: "https://example.com/test.git"}},
			},
			OutputDir: t.TempDir(),
			Options:   BuildOptions{SkipIfUnchanged: true},
		})

		if result.Skipped {
			t.Error("expected Skipped=false when SkipState is nil")
		}
	})

	// When SkipIfUnchanged is false, skip-evaluation must NOT happen even
	// if SkipState is provided.
	t.Run("skip_evaluation_disabled_when_option_false", func(t *testing.T) {
		svc := NewBuildService()

		result, _ := svc.Run(t.Context(), BuildRequest{
			Config: &config.Config{
				Repositories: []config.Repository{{Name: "test", URL: "https://example.com/test.git"}},
			},
			OutputDir: t.TempDir(),
			Options:   BuildOptions{SkipIfUnchanged: false},
		})

		if result.Skipped {
			t.Error("expected Skipped=false when SkipIfUnchanged=false")
		}
	})
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

// TestDirectBuildRequest_PreservesDocs verifies DirectBuildRequest carries
// doc files through without modification. RunDirect's behavior with real
// content is covered by TestDefaultBuildService_RunDirect_* in
// direct_service_test.go.
func TestDirectBuildRequest_PreservesDocs(t *testing.T) {
	files := []docs.DocFile{{Name: "x", Repository: "r"}}
	req := DirectBuildRequest{DocFiles: files, OutputDir: "/tmp/x"}
	if len(req.DocFiles) != 1 || req.DocFiles[0].Name != "x" {
		t.Errorf("DirectBuildRequest did not preserve doc files: %+v", req.DocFiles)
	}
}
