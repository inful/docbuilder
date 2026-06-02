package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestGenerateIndexPages(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)
	// Use lowercase repository names: docs.HugoContentPath lowercases the
	// repository component, so the index files are written under
	// content/repoa/ (not content/repoA/). On case-sensitive filesystems
	// (Linux CI), the test must read from the same lowercased directory.
	files := []docs.DocFile{
		{Repository: "repoa", Name: "alpha", RelativePath: "alpha.md", DocsBase: "docs", Section: "section1", Extension: ".md", Content: []byte("A")},
		{Repository: "repoa", Name: "beta", RelativePath: "beta.md", DocsBase: "docs", Section: "section1", Extension: ".md", Content: []byte("B")},
		{Repository: "repoa", Name: "root", RelativePath: "root.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("R")},
		{Repository: "repob", Name: "intro", RelativePath: "intro.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("I")},
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
	if !strings.Contains(string(b), "repoa") || !strings.Contains(string(b), "repob") {
		t.Fatalf("main index missing repo links: %s", string(b))
	}

	// Repo index
	repoIdx := filepath.Join(out, "content", "repoa", "_index.md")
	// #nosec G304 -- test utility reading from test output directory
	rb, err := os.ReadFile(repoIdx)
	if err != nil {
		t.Fatalf("read repo index: %v", err)
	}
	if !strings.Contains(string(rb), "## Sections") {
		t.Fatalf("repo index missing sections header: %s", string(rb))
	}
	if !strings.Contains(string(rb), "alpha/") || !strings.Contains(string(rb), "beta/") {
		t.Fatalf("repo index missing file links: %s", string(rb))
	}
	if !strings.Contains(string(rb), "## Section1") {
		t.Fatalf("repo index missing section1 subheader: %s", string(rb))
	}

	// Section index
	secIdx := filepath.Join(out, "content", "repoa", "section1", "_index.md")
	// #nosec G304 -- test utility reading from test output directory
	sb, err := os.ReadFile(secIdx)
	if err != nil {
		t.Fatalf("read section index: %v", err)
	}
	if !strings.Contains(string(sb), "Alpha") || !strings.Contains(string(sb), "Beta") {
		t.Fatalf("section index missing file entries: %s", string(sb))
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
