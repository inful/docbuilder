package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// newDaemonTestRepo creates a git repository with one markdown file and
// returns its path.
func newDaemonTestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	docsDir := filepath.Join(repoDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "a.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("docs/a.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@e", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return repoDir
}

// A daemon build must record the per-repository commit. The build job
// carries the repositories (in forge/discovery mode the config does not),
// so the state must be written for the repositories that were built.
func TestDaemonBuild_RecordsPerRepoCommit(t *testing.T) {
	out := t.TempDir()
	repoDir := newDaemonTestRepo(t)

	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			Storage: config.StorageConfig{RepoCacheDir: filepath.Join(out, "repos")},
		},
		Build: config.BuildConfig{
			WorkspaceDir:    filepath.Join(out, "ws"),
			CloneStrategy:   config.CloneStrategyFresh,
			SkipIfUnchanged: true,
		},
		Output: config.OutputConfig{Directory: filepath.Join(out, "site"), Clean: true},
	}

	svcResult := state.NewService(filepath.Join(out, "state"))
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	sm := state.NewServiceAdapter(svcResult.Unwrap())

	d := daemonForTest(t, daemonForTestOpts{
		Config:       cfg,
		BuildService: build.NewBuildService(),
		StateManager: sm,
	})

	job := &queue.BuildJob{
		ID:   "job-1",
		Type: queue.BuildTypeWebhook,
		TypedMeta: &queue.BuildJobMetadata{
			V2Config: cfg,
			Repositories: []config.Repository{{
				Name: "repoA", Paths: []string{"docs"}, URL: repoDir,
			}},
		},
	}

	report, err := d.Build(t.Context(), job)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if report == nil {
		t.Fatal("expected a build report")
	}

	rs := sm.GetRepository(repoDir)
	if rs == nil {
		t.Fatal("expected a per-repository state entry")
	}
	if rs.LastCommit == "" {
		t.Error("expected the repository commit to be recorded")
	}
	if rs.DocFilesHash == "" {
		t.Error("expected the repository doc-files hash to be recorded")
	}
}
