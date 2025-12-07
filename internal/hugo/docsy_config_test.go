package hugo

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestDocsyConfigGeneration verifies module import and JSON outputs for Docsy theme.
func TestDocsyConfigGeneration(t *testing.T) {
    cfg := &config.Config{}
    cfg.Hugo.Theme = "docsy"
    // Provide a minimal repository to allow github_repo param inference
    cfg.Repositories = []config.Repository{{URL: "https://github.com/example/repo", Name: "example", Branch: "main", Paths: []string{"docs"}}}

    g := NewGenerator(cfg, t.TempDir())
    if err := g.beginStaging(); err != nil {
        t.Fatalf("beginStaging: %v", err)
    }
    defer g.abortStaging()

    if err := g.generateHugoConfig(); err != nil {
        t.Fatalf("generateHugoConfig: %v", err)
    }

    // Validate cached features reflect Docsy
    if g.cachedThemeFeatures == nil {
        t.Fatalf("expected cachedThemeFeatures to be set")
    }
    feats := *g.cachedThemeFeatures
    if feats.Name != cfg.Hugo.ThemeType() {
        t.Fatalf("expected features.Name=%q, got %q", cfg.Hugo.ThemeType(), feats.Name)
    }

    // Read produced config
    cfgPath := filepath.Join(g.buildRoot(), "hugo.yaml")
    b, err := os.ReadFile(cfgPath)
    if err != nil {
        t.Fatalf("read hugo.yaml: %v", err)
    }
    content := string(b)

    // Assert Docsy uses modules and module import present
    if feats.UsesModules {
        if !strings.Contains(content, "module:") || !strings.Contains(content, feats.ModulePath) {
            t.Fatalf("expected module imports containing %q", feats.ModulePath)
        }
    }

    // Assert JSON outputs enabled when offline search feature is true
    if feats.EnableOfflineSearchJSON {
        if !strings.Contains(content, "JSON") {
            t.Fatalf("expected outputs to include JSON for offline search")
        }
    }

    // Assert key Docsy params present (search and github_repo inferred)
    if !strings.Contains(content, "search:") {
        t.Fatalf("expected search params to be present")
    }
    if !strings.Contains(content, "github_repo:") {
        t.Fatalf("expected github_repo param to be present")
    }
}
