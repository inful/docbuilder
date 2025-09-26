package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestGenerateSite_Smoke validates that a minimal set of doc files are processed
// and written with front matter and link rewriting applied. It does not invoke the external hugo binary.
func TestGenerateSite_Smoke(t *testing.T) {
	// Arrange temporary output dir
	outDir := t.TempDir()

	cfg := &config.V2Config{
		Hugo: config.HugoConfig{Title: "Test Site", Theme: "hextra"},
		Repositories: []config.Repository{
			{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}},
		},
	}

	// Fake discovered docs (normally discovered by Discovery)
	files := []docs.DocFile{
		{Repository: "repo1", Name: "intro", RelativePath: "intro.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("# Intro\n\nSee [Guide](guide.md).")},
		{Repository: "repo1", Name: "guide", RelativePath: "guide.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("# Guide\n")},
	}

	gen := NewGenerator(cfg, outDir)

	// Act
	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("GenerateSite failed: %v", err)
	}

	// Assert expected content files exist
	introPath := filepath.Join(outDir, "content", "repo1", "intro.md")
	guidePath := filepath.Join(outDir, "content", "repo1", "guide.md")
	if _, err := os.Stat(introPath); err != nil {
		t.Fatalf("intro file missing: %v", err)
	}
	if _, err := os.Stat(guidePath); err != nil {
		t.Fatalf("guide file missing: %v", err)
	}

	b, err := os.ReadFile(introPath)
	if err != nil {
		t.Fatalf("read intro: %v", err)
	}
	content := string(b)

	// Front matter delimiter
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("front matter missing at start: %s", content[:30])
	}
	if !strings.Contains(content, "repository: repo1") {
		t.Fatalf("repository not in front matter: %s", content)
	}
	if !strings.Contains(content, "editURL:") {
		t.Fatalf("expected editURL in front matter for hextra")
	}
	if strings.Contains(content, "guide.md") {
		t.Fatalf("link rewriting failed to strip .md: %s", content)
	}

	// Ensure date field present (format not strictly validated here for simplicity)
	if !strings.Contains(content, "date:") {
		t.Fatalf("date missing in front matter")
	}

	// Sanity: guide file also has front matter
	gb, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("read guide: %v", err)
	}
	if !strings.HasPrefix(string(gb), "---\n") {
		t.Fatalf("guide front matter missing")
	}
}

// Guard against accidental reliance on real time by verifying that BuildFrontMatter uses injected time.
func TestBuildFrontMatter_TimeInjection(t *testing.T) {
	ts := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	file := docs.DocFile{Repository: "repo", Name: "page"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: ts})
	if v, _ := fm["date"].(string); !strings.Contains(v, "2030-01-02") {
		t.Fatalf("expected injected date to include 2030-01-02, got %v", v)
	}
}
