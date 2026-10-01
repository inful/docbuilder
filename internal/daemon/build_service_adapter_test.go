package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
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

func (m *mockBuildService) RunDirect(ctx context.Context, req build.DirectBuildRequest) (*build.BuildResult, error) {
	return &build.BuildResult{
		Status:         build.BuildStatusSuccess,
		Repositories:   1,
		FilesProcessed: len(req.DocFiles),
		Duration:       time.Second,
		StartTime:      time.Now().Add(-time.Second),
		EndTime:        time.Now(),
	}, nil
}

// newDaemonForTest builds a minimal *Daemon suitable for exercising Build().
// Only buildSvc and buildMu are touched by Build(), so the other fields stay
// zero. Tests must not call anything that reads them.
func newDaemonForTest(svc build.BuildService) *Daemon {
	return &Daemon{
		buildSvc: svc,
	}
}

func TestDaemon_Build_NilJob(t *testing.T) {
	d := newDaemonForTest(&mockBuildService{})

	report, err := d.Build(t.Context(), nil)
	require.Error(t, err)
	require.Nil(t, report)
}

func TestDaemon_Build_MissingConfig(t *testing.T) {
	d := newDaemonForTest(&mockBuildService{})

	job := &queue.BuildJob{
		ID:        "test",
		TypedMeta: &queue.BuildJobMetadata{},
	}
	report, err := d.Build(t.Context(), job)
	require.Error(t, err)
	require.Nil(t, report)
}

func TestDaemon_Build_Success(t *testing.T) {
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
	d := newDaemonForTest(svc)

	job := &queue.BuildJob{
		ID: "test-job",
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{
				Output: config.OutputConfig{Directory: "/tmp/test"},
			},
		},
	}

	report, err := d.Build(t.Context(), job)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Equal(t, models.OutcomeSuccess, report.Outcome)
	require.Equal(t, 2, report.Repositories)
	require.Equal(t, 15, report.Files)
}

func TestDaemon_Build_CancellationPropagates(t *testing.T) {
	svc := &mockBuildService{
		runFunc: func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error) {
			return &build.BuildResult{
				Status: build.BuildStatusCancelled,
				Report: &models.BuildReport{Outcome: models.OutcomeCanceled},
			}, context.Canceled
		},
	}
	d := newDaemonForTest(svc)

	job := &queue.BuildJob{
		ID:        "test-job",
		TypedMeta: &queue.BuildJobMetadata{V2Config: &config.Config{}},
	}

	_, err := d.Build(t.Context(), job)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}

func TestDaemon_Build_Skipped(t *testing.T) {
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
	d := newDaemonForTest(svc)

	job := &queue.BuildJob{
		ID:        "test-job",
		TypedMeta: &queue.BuildJobMetadata{V2Config: &config.Config{}},
	}

	report, err := d.Build(t.Context(), job)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Equal(t, "no changes detected", report.SkipReason)
}

func TestDaemon_Build_WebhookOverridesRepositories(t *testing.T) {
	const repoName = "go-test-project"
	const repoURL = "https://git.home.luguber.info/inful/" + repoName + ".git"

	svc := &mockBuildService{
		runFunc: func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error) {
			if req.Config == nil {
				t.Fatal("expected non-nil config")
			}
			if len(req.Config.Repositories) != 1 {
				t.Fatalf("expected 1 repository, got %d", len(req.Config.Repositories))
			}
			if req.Config.Repositories[0].Name != repoName {
				t.Fatalf("unexpected repo name: %q", req.Config.Repositories[0].Name)
			}
			return &build.BuildResult{
				Status: build.BuildStatusSuccess,
				Report: &models.BuildReport{Outcome: models.OutcomeSuccess},
			}, nil
		},
	}
	d := newDaemonForTest(svc)

	job := &queue.BuildJob{
		ID:   "test-job",
		Type: queue.BuildTypeWebhook,
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{},
			Repositories: []config.Repository{{
				Name:   repoName,
				URL:    repoURL,
				Branch: "main",
				Paths:  []string{"docs"},
			}},
		},
	}

	report, err := d.Build(t.Context(), job)
	require.NoError(t, err)
	require.NotNil(t, report)
}

func TestJobToBuildRequest_MissingConfig(t *testing.T) {
	job := &queue.BuildJob{ID: "x"} // no TypedMeta → no V2Config
	_, err := jobToBuildRequest(job)
	require.Error(t, err)
}

func TestJobToBuildRequest_RepoOverride(t *testing.T) {
	const repoName = "ad-hoc"
	job := &queue.BuildJob{
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{},
			Repositories: []config.Repository{{
				Name:   repoName,
				URL:    "https://example.com/" + repoName + ".git",
				Branch: "main",
			}},
			RepoSnapshot: map[string]string{
				"https://example.com/" + repoName + ".git": "abc123",
			},
		},
	}
	req, err := jobToBuildRequest(job)
	require.NoError(t, err)
	require.Len(t, req.Config.Repositories, 1)
	require.Equal(t, repoName, req.Config.Repositories[0].Name)
	require.Equal(t, "abc123", req.Config.Repositories[0].PinnedCommit)
	require.True(t, req.Incremental)
	require.Equal(t, "./site", req.OutputDir)
	require.False(t, req.KeepWorkspace)
}

func TestJobToBuildRequest_PassesSkipIfUnchangedFromConfig(t *testing.T) {
	job := &queue.BuildJob{
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{
				Build: config.BuildConfig{SkipIfUnchanged: true},
			},
		},
	}
	req, err := jobToBuildRequest(job)
	require.NoError(t, err)
	require.True(t, req.Options.SkipIfUnchanged)
}

func TestResolveOutputDir(t *testing.T) {
	tests := []struct {
		name         string
		baseDir      string
		dir          string
		expectedPath string
		shouldBeAbs  bool
	}{
		{
			name:         "base_directory with relative directory",
			baseDir:      "/data",
			dir:          "site",
			expectedPath: "/data/site",
			shouldBeAbs:  true,
		},
		{
			name:         "base_directory with absolute directory (abs wins)",
			baseDir:      "/data",
			dir:          "/custom/site",
			expectedPath: "/custom/site",
			shouldBeAbs:  true,
		},
		{
			name:         "no base_directory with relative directory",
			baseDir:      "",
			dir:          "site",
			expectedPath: "site",
			shouldBeAbs:  false,
		},
		{
			name:         "no base_directory with absolute directory",
			baseDir:      "",
			dir:          "/var/site",
			expectedPath: "/var/site",
			shouldBeAbs:  true,
		},
		{
			name:         "empty directory with base_directory",
			baseDir:      "/data",
			dir:          "",
			expectedPath: "/data/site", // Default is "./site"
			shouldBeAbs:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Output: config.OutputConfig{
					BaseDirectory: tt.baseDir,
					Directory:     tt.dir,
				},
			}
			got := resolveOutputDir(cfg)
			if tt.shouldBeAbs && !filepath.IsAbs(got) {
				t.Errorf("Expected absolute path, got relative: %s", got)
			}
			if got != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, got)
			}
		})
	}
}
