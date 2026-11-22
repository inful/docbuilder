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

// addDocToRepo adds a markdown file to the test repository and commits it.
func addDocToRepo(t *testing.T, repoDir, name, content string) {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	full := filepath.Join(repoDir, "docs", name)
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	if _, err := wt.Add(filepath.Join("docs", name)); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("add "+name, &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@e", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// A build after the repository changed must not be skipped: the site has to
// show the new content.
func TestBuild_AfterChangeIsNotSkipped(t *testing.T) {
	out := t.TempDir()
	repoDir := newTestRepo(t)

	cfg := &appcfg.Config{
		Output: appcfg.OutputConfig{Directory: out, Clean: true},
		Build: appcfg.BuildConfig{
			WorkspaceDir:    filepath.Join(out, "ws"),
			CloneStrategy:   appcfg.CloneStrategyFresh,
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
		StateManager: sm,
	}

	first, err := svc.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("build 1: %v", err)
	}
	if first.Skipped {
		t.Fatal("the first build must not be skipped")
	}
	first.Report.Outcome = models.OutcomeSuccess
	sm.SetLastConfigHash(first.Report.ConfigHash)
	sm.SetLastGlobalDocFilesHash(first.Report.DocFilesHash)

	// The repository gets a new document.
	addDocToRepo(t, repoDir, "b.md", "# B\n")

	second, err := svc.Run(t.Context(), req)
	if err != nil {
		t.Fatalf("build 2: %v", err)
	}
	if second.Skipped {
		t.Fatal("the build after a change must not be skipped")
	}
}
