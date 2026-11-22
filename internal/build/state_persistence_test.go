package build

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// newTestRepo creates a git repository with one markdown file and returns
// its path.
func newTestRepo(t *testing.T) string {
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

// A build must persist the per-repository state (commit, document hash).
// Without it the next build cannot tell whether anything changed, so
// skip-evaluation never fires.
func TestBuild_PersistsPerRepoState(t *testing.T) {
	out := t.TempDir()
	repoDir := newTestRepo(t)

	cfg := &appcfg.Config{
		Output: appcfg.OutputConfig{Directory: out, Clean: true},
		Build: appcfg.BuildConfig{
			WorkspaceDir:    filepath.Join(out, "ws"),
			CloneStrategy:   appcfg.CloneStrategyFresh,
			SkipIfUnchanged: true,
		},
		Repositories: []appcfg.Repository{{
			Name: "repoA", Paths: []string{"docs"}, URL: repoDir,
		}},
	}

	svcResult := state.NewService(out)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	sm := state.NewServiceAdapter(svcResult.Unwrap())

	svc := NewBuildService()
	req := BuildRequest{
		Config:       cfg,
		OutputDir:    out,
		SkipState:    sm,
		StateManager: sm,
		Options:      BuildOptions{SkipIfUnchanged: true},
	}
	res, err := svc.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if res.Report.Repositories != 1 {
		t.Fatalf("expected 1 repository, got %d", res.Report.Repositories)
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

// The second build of an unchanged repository must be skipped.
func TestBuild_SkipsUnchangedRepository(t *testing.T) {
	out := t.TempDir()
	repoDir := newTestRepo(t)

	cfg := &appcfg.Config{
		Output: appcfg.OutputConfig{Directory: out, Clean: true},
		Build: appcfg.BuildConfig{
			WorkspaceDir:  filepath.Join(out, "ws"),
			CloneStrategy: appcfg.CloneStrategyFresh,
			// "always" runs Hugo, so the site is rendered and the next
			// build can be skipped. "auto" renders nothing.
			RenderMode:      appcfg.RenderModeAlways,
			SkipIfUnchanged: true,
		},
		Repositories: []appcfg.Repository{{
			Name: "repoA", Paths: []string{"docs"}, URL: repoDir,
		}},
	}

	svcResult := state.NewService(out)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	sm := state.NewServiceAdapter(svcResult.Unwrap())

	svc := NewBuildService()
	req := BuildRequest{
		Config:       cfg,
		OutputDir:    out,
		SkipState:    sm,
		StateManager: sm,
		Options:      BuildOptions{SkipIfUnchanged: true},
	}

	first, err := svc.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("build 1: %v", err)
	}
	if first.Skipped {
		t.Fatal("the first build must not be skipped")
	}

	// The daemon records the config hash and the global doc-files hash
	// after a successful build; skip-evaluation compares against them.
	first.Report.Outcome = models.OutcomeSuccess
	sm.SetLastConfigHash(first.Report.ConfigHash)
	sm.SetLastGlobalDocFilesHash(first.Report.DocFilesHash)

	second, err := svc.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("build 2: %v", err)
	}
	if !second.Skipped {
		t.Errorf("expected the unchanged second build to be skipped, outcome=%s", second.Report.Outcome)
	}
}
