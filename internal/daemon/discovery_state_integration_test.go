package daemon

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"

    cfg "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/hugo"
    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing/object"
)

// TestDiscoveryStagePersistsPerRepoDocFilesHash exercises the public GenerateFullSite API
// with an attached state manager to verify that per-repository document counts and
// doc_files_hash are persisted during the discovery stage. It uses a real local git
// repository (cloned via local path) to avoid import cycles with internal stages.
func TestDiscoveryStagePersistsPerRepoDocFilesHash(t *testing.T) {
    if testing.Short() { t.Skip("short") }
    tmp := t.TempDir()
    outputDir := filepath.Join(tmp, "out")
    workspace := filepath.Join(tmp, "workspace")
    if err := os.MkdirAll(workspace, 0o755); err != nil { t.Fatalf("mkdir workspace: %v", err) }

    // Initialize a local git repository with one markdown file in docs/.
    remote := filepath.Join(tmp, "remote-repo-one")
    if err := os.MkdirAll(filepath.Join(remote, "docs"), 0o755); err != nil { t.Fatalf("mkdir docs: %v", err) }
    if err := os.WriteFile(filepath.Join(remote, "docs", "page.md"), []byte("# Page\n"), 0o644); err != nil { t.Fatalf("write page: %v", err) }
    repo, err := git.PlainInit(remote, false)
    if err != nil { t.Fatalf("init repo: %v", err) }
    wt, err := repo.Worktree(); if err != nil { t.Fatalf("worktree: %v", err) }
    if _, err := wt.Add("docs/page.md"); err != nil { t.Fatalf("add: %v", err) }
    if _, err := wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "dev", Email: "dev@example.com", When: time.Now()}}); err != nil { t.Fatalf("commit: %v", err) }

    // Configuration referencing the local path as clone URL.
    repository := cfg.Repository{ Name: "repo-one", Paths: []string{"docs"}, URL: remote, Branch: "", Tags: map[string]string{"forge_type": "github"} }
    conf := &cfg.Config{ Output: cfg.OutputConfig{ Directory: outputDir, Clean: true }, Build: cfg.BuildConfig{ WorkspaceDir: workspace, CloneStrategy: cfg.CloneStrategyFresh, NamespaceForges: cfg.NamespacingAuto }, Hugo: cfg.HugoConfig{ Theme: "hextra" }, Repositories: []cfg.Repository{ repository } }

    sm, err := NewStateManager(tmp); if err != nil { t.Fatalf("state manager: %v", err) }
    gen := hugo.NewGenerator(conf, outputDir).WithStateManager(sm)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
    report, err := gen.GenerateFullSite(ctx, conf.Repositories, workspace)
    if err != nil { t.Fatalf("GenerateFullSite: %v", err) }
    if report == nil { t.Fatalf("expected report") }

    rs := sm.GetRepository(repository.URL)
    if rs == nil { t.Fatalf("expected repository state for %s", repository.URL) }
    if rs.DocumentCount != 1 { t.Fatalf("expected document_count=1 got %d", rs.DocumentCount) }
    if rs.DocFilesHash == "" { t.Fatalf("expected non-empty doc_files_hash") }
    if report.DocFilesHash == "" { t.Fatalf("expected build report doc_files_hash set") }
    if rs.DocFilesHash != report.DocFilesHash { t.Fatalf("per-repo hash %s != report hash %s (single-repo build)", rs.DocFilesHash, report.DocFilesHash) }
}
