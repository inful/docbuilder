package hugo

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/docs"
)

// helper to read file content quickly
func mustRead(t *testing.T, path string) string {
    b, err := os.ReadFile(path)
    if err != nil { t.Fatalf("read %s: %v", path, err) }
    return string(b)
}

// TestAtomicStaging_SuccessPromotesNewContent verifies that a second successful build replaces content atomically
// and leaves no staging directories behind.
func TestAtomicStaging_SuccessPromotesNewContent(t *testing.T) {
    outDir := t.TempDir()
    cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
    gen := NewGenerator(cfg, outDir)

    // First build v1
    filesV1 := []docs.DocFile{{Repository: "repo", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Version1\n")}}
    if err := gen.GenerateSite(filesV1); err != nil { t.Fatalf("first build failed: %v", err) }
    target := filepath.Join(outDir, "content", "repo", "page.md")
    v1 := mustRead(t, target)
    if !strings.Contains(v1, "Version1") { t.Fatalf("expected v1 content, got %s", v1) }

    // Second build v2
    gen2 := NewGenerator(cfg, outDir)
    filesV2 := []docs.DocFile{{Repository: "repo", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Version2\n")}}
    if err := gen2.GenerateSite(filesV2); err != nil { t.Fatalf("second build failed: %v", err) }
    v2 := mustRead(t, target)
    if !strings.Contains(v2, "Version2") { t.Fatalf("expected v2 content after promotion, got %s", v2) }
    if strings.Contains(v2, "Version1") { t.Fatalf("old content leaked into new file: %s", v2) }

    // Ensure no staging directories remain
    parent := filepath.Dir(outDir)
    entries, err := os.ReadDir(parent)
    if err != nil { t.Fatalf("readdir parent: %v", err) }
    base := filepath.Base(outDir) + ".staging-"
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), base) {
            t.Fatalf("found leftover staging directory: %s", e.Name())
        }
    }
}

// TestAtomicStaging_FailedBuildRetainsOldContent ensures that a failed (canceled) build does not replace existing output
// and that the staging directory is cleaned up.
func TestAtomicStaging_FailedBuildRetainsOldContent(t *testing.T) {
    outDir := t.TempDir()
    cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
    gen := NewGenerator(cfg, outDir)

    // Initial successful build
    filesV1 := []docs.DocFile{{Repository: "repo", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Stable\n")}}
    if err := gen.GenerateSite(filesV1); err != nil { t.Fatalf("initial build failed: %v", err) }
    target := filepath.Join(outDir, "content", "repo", "page.md")
    stable := mustRead(t, target)
    if !strings.Contains(stable, "Stable") { t.Fatalf("expected stable content, got %s", stable) }

    // Start second build with immediate cancellation
    gen2 := NewGenerator(cfg, outDir)
    filesV2 := []docs.DocFile{{Repository: "repo", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Broken\n")}}
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // cancel immediately
    if _, err := gen2.GenerateSiteWithReportContext(ctx, filesV2); err == nil { t.Fatalf("expected cancellation error") }

    // Old content should remain
    after := mustRead(t, target)
    if !strings.Contains(after, "Stable") { t.Fatalf("stable content lost after failed build: %s", after) }
    if strings.Contains(after, "Broken") { t.Fatalf("new (failed) content appeared in final output: %s", after) }

    // Allow brief time for any async cleanup (prev removal) then check no staging dirs.
    time.Sleep(20 * time.Millisecond)
    parent := filepath.Dir(outDir)
    entries, err := os.ReadDir(parent)
    if err != nil { t.Fatalf("readdir parent: %v", err) }
    base := filepath.Base(outDir) + ".staging-"
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), base) {
            t.Fatalf("leftover staging directory after failed build: %s", e.Name())
        }
    }
}
