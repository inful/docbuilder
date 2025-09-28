package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestDaemonStateBuildCounters ensures that after a full build the state manager records per-repo build counts > 0.
func TestDaemonStateBuildCounters(t *testing.T) {
	// Skip in short mode due to filesystem + hugo invocation.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp output/workspace dirs
	out := t.TempDir()
	ws := filepath.Join(out, "_ws")
	if err := os.MkdirAll(ws, 0o755); err != nil { t.Fatalf("mkdir ws: %v", err) }

	// Create a fake repository with docs
	repoDir := filepath.Join(ws, "repoA")
	if err := os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755); err != nil { t.Fatalf("mkdir repo docs: %v", err) }
	if err := os.WriteFile(filepath.Join(repoDir, "docs", "page.md"), []byte("# Page\n"), 0o644); err != nil { t.Fatalf("write doc: %v", err) }
	// Initialize minimal git structure to satisfy head reading (optional); we can stub by writing .git/HEAD with dummy hash.
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil { t.Fatalf("mkdir git: %v", err) }
	if err := os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("dummyhash"), 0o644); err != nil { t.Fatalf("write head: %v", err) }

	config := &cfg.Config{Output: cfg.OutputConfig{Directory: out}, Build: cfg.BuildConfig{WorkspaceDir: ws}, Hugo: cfg.HugoConfig{Theme: "hextra"}}
	repo := cfg.Repository{Name: "repoA", Paths: []string{"docs"}, URL: "https://example.com/repoA.git", Branch: "main"}
	config.Repositories = []cfg.Repository{repo}

	gen := hugo.NewGenerator(config, out)
	// Instead of triggering full clone stage we directly supply doc files using GenerateSiteWithReport.
	files := []docs.DocFile{{Repository: repo.Name, Name: "page", Extension: ".md", DocsBase: "docs", RelativePath: "page.md"}}
	if err := files[0].LoadContent(); err == nil { /* content loaded if path real */ }
	if err := gen.GenerateSite(files); err != nil { t.Fatalf("generate site: %v", err) }

	// Attach state manager manually and run builder logic to update counts (simulate daemon builder behavior)
	sm, err := NewStateManager(out)
	if err != nil { t.Fatalf("state manager: %v", err) }
	builder := NewSiteBuilder()
	job := &BuildJob{Metadata: map[string]any{"v2_config": config, "repositories": []cfg.Repository{repo}, "state_manager": sm}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := builder.Build(ctx, job); err != nil { t.Fatalf("builder.Build: %v", err) }

	rs := sm.GetRepository(repo.URL)
	if rs == nil { t.Fatalf("expected repository state created") }
	if rs.BuildCount == 0 { t.Fatalf("expected build count > 0, got %d", rs.BuildCount) }
}
