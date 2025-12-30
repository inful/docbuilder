package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// TestDaemonStateBuildCounters ensures that after a full build the state manager records per-repo build counts > 0.
func TestDaemonStateBuildCounters(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	out := t.TempDir()
	ws := filepath.Join(out, "workspace")
	if err := os.MkdirAll(ws, 0o750); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	// Initialize a real local git repository so clone stage succeeds.
	repoDir := filepath.Join(out, "remote-repoA")
	if err := os.MkdirAll(filepath.Join(repoDir, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "docs", "page.md"), []byte("# Page\n"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	r, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	wt, err := r.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, addErr := wt.Add("docs/page.md"); addErr != nil {
		t.Fatalf("add: %v", addErr)
	}
	if _, commitErr := wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "tester", Email: "tester@example.com", When: time.Now()}}); commitErr != nil {
		t.Fatalf("commit: %v", commitErr)
	}

	// Use absolute path as clone URL (supported by go-git for local clone).
	// Leave Branch empty to allow cloning default branch created by PlainInit (typically 'master').
	repo := cfg.Repository{Name: "repoA", Paths: []string{"docs"}, URL: repoDir, Branch: ""}
	config := &cfg.Config{Output: cfg.OutputConfig{Directory: out, Clean: true}, Build: cfg.BuildConfig{WorkspaceDir: ws, CloneStrategy: cfg.CloneStrategyFresh}, Hugo: cfg.HugoConfig{}, Repositories: []cfg.Repository{repo}}

	// Use typed state.ServiceAdapter instead of legacy StateManager
	svcResult := state.NewService(out)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	sm := state.NewServiceAdapter(svcResult.Unwrap())
	sm.EnsureRepositoryState(repo.URL, repo.Name, repo.Branch)
	gen := hugo.NewGenerator(config, out).WithStateManager(sm).WithRenderer(&hugo.NoopRenderer{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := gen.GenerateFullSite(ctx, []cfg.Repository{repo}, ws)
	if err != nil {
		t.Fatalf("GenerateFullSite: %v", err)
	}
	if report == nil {
		t.Fatalf("nil report")
	}
	// Simulate builder increment (since we bypassed builder).
	sm.IncrementRepoBuild(repo.URL, true)
	rs := sm.GetRepository(repo.URL)
	if rs == nil {
		t.Fatalf("expected repository state created")
	}
	if rs.DocumentCount != 1 {
		t.Fatalf("expected document count=1 got %d", rs.DocumentCount)
	}
	if rs.BuildCount == 0 {
		t.Fatalf("expected build count >0 got %d", rs.BuildCount)
	}
}
