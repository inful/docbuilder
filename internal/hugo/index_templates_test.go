package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// helper to build minimal generator with output dir
func newTestGenerator(t *testing.T) *Generator {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{Hugo: config.HugoConfig{Title: "Test Site", Description: "Desc", Theme: "hextra"}}
	return NewGenerator(cfg, tmp)
}

func TestIndexTemplates_FallbackEmbedded(t *testing.T) {
	g := newTestGenerator(t)
	files := []docs.DocFile{{Repository: "repo1", Name: "doc1", Path: "doc1.md"}}
	// create expected content root
	if err := os.MkdirAll(filepath.Join(g.outputDir, "content"), 0o755); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := g.generateIndexPages(files); err != nil {
		t.Fatalf("generate indexes: %v", err)
	}
	mainIdx := filepath.Join(g.outputDir, "content", "_index.md")
	b, err := os.ReadFile(mainIdx)
	if err != nil {
		t.Fatalf("read main index: %v", err)
	}
	if !strings.Contains(string(b), "# Test Site") {
		t.Fatalf("expected embedded template output, got:\n%s", string(b))
	}
}

func TestIndexTemplates_UserOverridePrecedence(t *testing.T) {
	g := newTestGenerator(t)
	// create override in highest precedence path
	overrideDir := filepath.Join(g.outputDir, "templates", "index")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("mkdir override: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(g.outputDir, "content"), 0o755); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	content := "CUSTOM MAIN TEMPLATE\n"
	if err := os.WriteFile(filepath.Join(overrideDir, "main.md.tmpl"), []byte(content), 0o600); err != nil {
		t.Fatalf("write override: %v", err)
	}
	files := []docs.DocFile{{Repository: "repo1", Name: "doc1", Path: "doc1.md"}}
	if err := g.generateIndexPages(files); err != nil {
		t.Fatalf("generate indexes: %v", err)
	}
	mainIdx := filepath.Join(g.outputDir, "content", "_index.md")
	b, err := os.ReadFile(mainIdx)
	if err != nil {
		t.Fatalf("read main index: %v", err)
	}
	if !strings.Contains(string(b), "CUSTOM MAIN TEMPLATE") {
		t.Fatalf("expected override template content, got:\n%s", string(b))
	}
}
