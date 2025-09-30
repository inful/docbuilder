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

// TestPipelineGolden validates stable output of the transform pipeline for a simple page.
func TestPipelineGolden(t *testing.T) {
    cfg := &config.Config{}
    gen := &Generator{config: cfg}

    md := "---\ncustom: val\n---\nBody with [rel](./ref.md) and more."
    file := docs.DocFile{Path: "repo/docs/page.md", RelativePath: "repo/docs/page.md", Repository: "repo", Name: "page", Extension: ".md", Content: []byte(md)}
    _ = file.LoadContent()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    t.Cleanup(cancel)
    if err := gen.copyContentFiles(ctx, []docs.DocFile{file}); err != nil {
        t.Fatalf("pipeline run failed: %v", err)
    }

    out := filepath.Join(gen.buildRoot(), file.GetHugoPath())
    b, err := os.ReadFile(out)
    if err != nil {
        t.Fatalf("read output: %v", err)
    }
    content := string(b)
    // Front matter expectations
    expectFM := []string{"custom: val", "title:"}
    for _, needle := range expectFM {
        if !strings.Contains(content, needle) {
            t.Fatalf("expected front matter to contain %q; got:\n%s", needle, content)
        }
    }
    // Link rewrite expectation (extension removed)
    if !strings.Contains(content, "[rel](./ref)") {
        t.Fatalf("expected relative link extension removed; got:\n%s", content)
    }
}
