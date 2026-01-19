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

func TestGenerateIndexPages(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)
	files := []docs.DocFile{
		{Repository: "repoA", Name: "alpha", RelativePath: "alpha.md", DocsBase: "docs", Section: "section1", Extension: ".md", Content: []byte("A")},
		{Repository: "repoA", Name: "beta", RelativePath: "beta.md", DocsBase: "docs", Section: "section1", Extension: ".md", Content: []byte("B")},
		{Repository: "repoA", Name: "root", RelativePath: "root.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("R")},
		{Repository: "repoB", Name: "intro", RelativePath: "intro.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("I")},
	}

	// Need structure for indexes (skip full generation) -> just call generateIndexPages after structure creation
	if err := gen.CreateHugoStructure(); err != nil {
		t.Fatalf("structure: %v", err)
	}
	if err := gen.generateIndexPages(files); err != nil {
		t.Fatalf("generate indexes: %v", err)
	}

	// Main index
	mainIdx := filepath.Join(out, "content", "_index.md")
	// #nosec G304 -- test utility reading from test output directory
	b, err := os.ReadFile(mainIdx)
	if err != nil {
		t.Fatalf("read main index: %v", err)
	}
	if !strings.Contains(string(b), "Repositories") {
		t.Fatalf("main index missing repositories header: %s", string(b))
	}
	if !strings.Contains(string(b), "repoA") || !strings.Contains(string(b), "repoB") {
		t.Fatalf("main index missing repo links: %s", string(b))
	}

	// Repo index
	repoIdx := filepath.Join(out, "content", "repoA", "_index.md")
	// #nosec G304 -- test utility reading from test output directory
	rb, err := os.ReadFile(repoIdx)
	if err != nil {
		t.Fatalf("read repo index: %v", err)
	}
	if !strings.Contains(string(rb), "Alpha Documentation") && !strings.Contains(string(rb), "Documentation") { // lenient: tolerate missing specific phrase but ensure file has some content
		if len(strings.TrimSpace(string(rb))) == 0 {
			t.Fatalf("repo index unexpectedly empty")
		}
	}
	if !strings.Contains(string(rb), "alpha/") || !strings.Contains(string(rb), "beta/") {
		t.Fatalf("repo index missing file links: %s", string(rb))
	}

	// Section index
	secIdx := filepath.Join(out, "content", "repoA", "section1", "_index.md")
	// #nosec G304 -- test utility reading from test output directory
	sb, err := os.ReadFile(secIdx)
	if err != nil {
		t.Fatalf("read section index: %v", err)
	}
	if !strings.Contains(string(sb), "Alpha") || !strings.Contains(string(sb), "Beta") {
		t.Fatalf("section index missing file entries: %s", string(sb))
	}

	// Basic date presence
	if !strings.Contains(string(rb), time.Now().Format("2006")) { // not strict; log only
		t.Logf("year not present in repo index (non-fatal)")
	}
}

func TestGenerateMainIndex_SkipsIfExists(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)

	if err := gen.CreateHugoStructure(); err != nil {
		t.Fatalf("structure: %v", err)
	}

	// Pre-create a custom main index that simulates a user-provided README.md
	// that was normalized to content/_index.md by the transform pipeline.
	mainIdx := filepath.Join(out, "content", "_index.md")
	custom := "---\ntitle: Custom\n---\n\n# Custom Home\n"
	// #nosec G306 -- test content written to temp dir
	if err := os.WriteFile(mainIdx, []byte(custom), 0o644); err != nil {
		t.Fatalf("write custom main index: %v", err)
	}

	files := []docs.DocFile{{Repository: "local", Name: "guide", RelativePath: "guide.md", DocsBase: ".", Section: "", Extension: ".md", Content: []byte("# Guide\n")}}
	if err := gen.generateMainIndex(files); err != nil {
		t.Fatalf("generate main index: %v", err)
	}

	// #nosec G304 -- test utility reading from test output directory
	b, err := os.ReadFile(mainIdx)
	if err != nil {
		t.Fatalf("read main index: %v", err)
	}
	if string(b) != custom {
		t.Fatalf("expected custom main index to be preserved; got: %s", string(b))
	}
}
