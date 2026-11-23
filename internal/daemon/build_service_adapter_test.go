package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// mockBuildService is a test double for build.BuildService.
type mockBuildService struct {
	runFunc func(ctx context.Context, req build.BuildRequest) (*build.BuildResult, error)
}

// Compile-time check: mockBuildService must implement the full build.BuildService
// interface, including RunDirect. If a new method is added to the interface,
// this assertion fails to compile (instead of being caught at test runtime).
var _ build.BuildService = (*mockBuildService)(nil)

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

// newDaemonForTest is a thin wrapper around daemonForTest for tests
// that only need the Build() entry point wired up. The BuildService
// is the only seam these tests exercise; every other sub-struct is
// initialized to its zero value (and most are nil so the test
// asserts on the absence of a seam).
//
// This helper exists for the legacy tests in this file. New tests
// should call daemonForTest directly with the seams they need.
func newDaemonForTest(t testingT, svc build.BuildService) *Daemon {
	return daemonForTest(t, daemonForTestOpts{
		Config:       &config.Config{Daemon: &config.DaemonConfig{}},
		BuildService: svc,
	})
}

// fakeDaemonStateManager satisfies state.DaemonStateManager (the aggregate
// interface the daemon stores in its stateManager field). All unused
// methods are no-op stubs; tests that exercise them should add a more
// capable fake.
type fakeDaemonStateManager struct {
	repoLastCommit map[string]string
}

func newFakeDaemonStateManager() *fakeDaemonStateManager {
	return &fakeDaemonStateManager{repoLastCommit: map[string]string{}}
}

func (f *fakeDaemonStateManager) GetRepoLastCommit(url string) string {
	return f.repoLastCommit[url]
}
func (*fakeDaemonStateManager) GetLastConfigHash() string         { return "" }
func (*fakeDaemonStateManager) SetLastConfigHash(string)          {}
func (*fakeDaemonStateManager) GetLastReportChecksum() string     { return "" }
func (*fakeDaemonStateManager) SetLastReportChecksum(string)      {}
func (*fakeDaemonStateManager) GetRepoDocFilesHash(string) string { return "" }
func (*fakeDaemonStateManager) GetRepoDocumentCount(string) int   { return 0 }
func (*fakeDaemonStateManager) GetLastGlobalDocFilesHash() string { return "" }
func (*fakeDaemonStateManager) SetLastGlobalDocFilesHash(string)  {}
func (f *fakeDaemonStateManager) SetRepoLastCommit(url, _, _, commit string) {
	f.repoLastCommit[url] = commit
}

func (*fakeDaemonStateManager) Load() error           { return nil }
func (*fakeDaemonStateManager) Save() error           { return nil }
func (*fakeDaemonStateManager) IsLoaded() bool        { return true }
func (*fakeDaemonStateManager) LastSaved() *time.Time { return nil }
func (*fakeDaemonStateManager) EnsureRepositoryState(string, string, string) {
}
func (*fakeDaemonStateManager) SetRepoDocumentCount(string, int)   {}
func (*fakeDaemonStateManager) SetRepoDocFilesHash(string, string) {}

func (*fakeDaemonStateManager) IncrementRepoBuild(string, bool) {}
func (*fakeDaemonStateManager) RecordDiscovery(string, int)     {}
func (*fakeDaemonStateManager) GetRepository(string) *state.RepositoryState {
	return nil
}

// Compile-time check that the fake satisfies state.DaemonStateManager.
// If the interface gains a new method, this fails to compile and forces
// the fake to be updated.
var _ state.DaemonStateManager = (*fakeDaemonStateManager)(nil)

func TestDaemon_Build_NilJob(t *testing.T) {
	d := newDaemonForTest(t, &mockBuildService{})

	report, err := d.Build(t.Context(), nil)
	require.Error(t, err)
	require.Nil(t, report)
}

func TestDaemon_Build_MissingConfig(t *testing.T) {
	d := newDaemonForTest(t, &mockBuildService{})

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
	d := newDaemonForTest(t, svc)

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
	d := newDaemonForTest(t, svc)

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
	d := newDaemonForTest(t, svc)

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
	d := newDaemonForTest(t, svc)

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
	d := newDaemonForTest(t, nil)
	job := &queue.BuildJob{ID: "x"} // no TypedMeta → no V2Config
	_, err := d.jobToBuildRequest(job)
	require.Error(t, err)
}

func TestJobToBuildRequest_RepoOverride(t *testing.T) {
	const repoName = "ad-hoc"
	d := newDaemonForTest(t, nil)
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
	req, err := d.jobToBuildRequest(job)
	require.NoError(t, err)
	require.Len(t, req.Config.Repositories, 1)
	require.Equal(t, repoName, req.Config.Repositories[0].Name)
	require.Equal(t, "abc123", req.Config.Repositories[0].PinnedCommit)
	require.True(t, req.Incremental)
	require.Equal(t, "./site", req.OutputDir)
	require.False(t, req.KeepWorkspace)
}

func TestJobToBuildRequest_PassesSkipIfUnchangedFromConfig(t *testing.T) {
	d := newDaemonForTest(t, nil)
	job := &queue.BuildJob{
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{
				Build: config.BuildConfig{SkipIfUnchanged: true},
			},
		},
	}
	req, err := d.jobToBuildRequest(job)
	require.NoError(t, err)
	require.True(t, req.Config.Build.SkipIfUnchanged,
		"the configuration decides whether a build may be skipped")
}

// TestJobToBuildRequest_SetsStateManagerFromStateManager verifies that the
// daemon passes its state manager to the canonical BuildService via
// req.StateManager. The pipeline records the commit each repository was
// built at there, and reads it back to see whether anything moved.
func TestJobToBuildRequest_SetsStateManagerFromStateManager(t *testing.T) {
	st := newFakeDaemonStateManager()
	d := daemonForTest(t, daemonForTestOpts{
		Config:       &config.Config{Daemon: &config.DaemonConfig{}},
		StateManager: st,
	})
	job := &queue.BuildJob{
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{
				Build: config.BuildConfig{SkipIfUnchanged: true},
			},
		},
	}
	req, err := d.jobToBuildRequest(job)
	require.NoError(t, err)
	require.Same(t, any(st), any(req.StateManager),
		"req.StateManager must be the daemon's state manager; otherwise the pipeline cannot tell whether a repository moved")
	require.True(t, req.Config.Build.SkipIfUnchanged,
		"sanity: SkipIfUnchanged from config must still be carried through")
}

// TestJobToBuildRequest_SetsOnDocumentReadyWhenDispatcherEnabled verifies
// that the outbound dispatcher's Enqueue method is installed as the
// per-document callback on the BuildRequest, so ragabast ingest receives
// documents as they are written. Regression test for the
// build-service-unification refactor: Phase 4 removed the HugoGeneratorFactory
// closure (which had wired WithDocumentReady inside the daemon path)
// without setting req.OnDocumentReady, silently disabling ragabast ingest.
func TestJobToBuildRequest_SetsOnDocumentReadyWhenDispatcherEnabled(t *testing.T) {
	dispatcher, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL:   "http://127.0.0.1:1",
		Workers:   1,
		QueueSize: 4,
	})
	require.NoError(t, err)

	d := daemonForTest(t, daemonForTestOpts{
		Config:             &config.Config{Daemon: &config.DaemonConfig{}},
		OutboundDispatcher: dispatcher,
	})
	job := &queue.BuildJob{
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{},
		},
	}
	req, err := d.jobToBuildRequest(job)
	require.NoError(t, err)
	require.NotNil(t, req.OnDocumentReady,
		"req.OnDocumentReady must be set when outbound dispatcher is enabled; "+
			"otherwise ragabast ingest never receives documents")
	// And the callback must be the dispatcher's Enqueue method (we
	// compare by reflect.Value.Pointer() because funcs are not directly
	// comparable but method values compare by identity).
	require.Equal(t,
		reflect.ValueOf(dispatcher.Enqueue).Pointer(),
		reflect.ValueOf(req.OnDocumentReady).Pointer(),
		"OnDocumentReady must be dispatcher.Enqueue")
}

// TestJobToBuildRequest_NoOnDocumentReadyWhenDispatcherDisabled is the
// inverse case: when the dispatcher is nil, OnDocumentReady must be nil
// so BuildService.Run does not pay for an unused WithDocumentReady call.
func TestJobToBuildRequest_NoOnDocumentReadyWhenDispatcherDisabled(t *testing.T) {
	d := newDaemonForTest(t, nil) // no outboundDispatcher
	job := &queue.BuildJob{
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: &config.Config{},
		},
	}
	req, err := d.jobToBuildRequest(job)
	require.NoError(t, err)
	require.Nil(t, req.OnDocumentReady,
		"req.OnDocumentReady must be nil when outbound dispatcher is disabled")
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
