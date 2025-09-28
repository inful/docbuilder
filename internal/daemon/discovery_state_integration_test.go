package daemon

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    cfg "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// TestDiscoveryStagePersistsPerRepoDocFilesHash ensures discovery stage writes per-repo
// document counts and doc_files_hash via the generator's attached state manager.
func TestDiscoveryStagePersistsPerRepoDocFilesHash(t *testing.T) {
    if testing.Short() { t.Skip("short mode") }
    base := t.TempDir()
    repoPath := filepath.Join(base, "repo-one")
    docsDir := filepath.Join(repoPath, "docs")
    if err := os.MkdirAll(docsDir, 0o755); err != nil { t.Fatalf("mkdir docs: %v", err) }
    if err := os.WriteFile(filepath.Join(docsDir, "page.md"), []byte("# Page\n"), 0o644); err != nil { t.Fatalf("write page: %v", err) }
    // Make fake git HEAD so clone/update logic (if invoked) has something to read; we bypass clone by pointing workspace directly.
    if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil { t.Fatalf("mkdir git: %v", err) }
    if err := os.WriteFile(filepath.Join(repoPath, ".git", "HEAD"), []byte("deadbeef"), 0o644); err != nil { t.Fatalf("write head: %v", err) }

    repo := cfg.Repository{Name: "repo-one", Paths: []string{"docs"}, URL: "https://example.com/repo-one.git", Branch: "main", Tags: map[string]string{"forge_type": "github"}}
    conf := &cfg.Config{Hugo: cfg.HugoConfig{Theme: "hextra"}, Build: cfg.BuildConfig{NamespaceForges: cfg.NamespacingAuto}}
    conf.Repositories = []cfg.Repository{repo}

    sm, err := NewStateManager(base)
    if err != nil { t.Fatalf("state manager: %v", err) }

    // We simulate a build by constructing the generator with state manager, then calling GenerateFullSite with a pre-created workspace.
    // Provide workspaceDir where repo already exists so clone stage will treat it as existing (auto strategy default) and update heads.
    conf.Build.WorkspaceDir = base
    gen := hugo.NewGenerator(conf, filepath.Join(base, "out")).WithStateManager(sm)
    ctx := context.Background()
    if _, err := gen.GenerateFullSite(ctx, []cfg.Repository{repo}, base); err != nil {
        t.Fatalf("GenerateFullSite: %v", err)
    }

    rs := sm.GetRepository(repo.URL)
    if rs == nil { t.Fatalf("expected repository state for %s", repo.URL) }
    if rs.DocumentCount != 1 { t.Fatalf("expected document_count=1 got %d", rs.DocumentCount) }
    if rs.DocFilesHash == "" { t.Fatalf("expected non-empty doc_files_hash") }
}
