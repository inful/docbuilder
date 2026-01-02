package hugo

import (
	"context"
	"os"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestPipeline_Idempotency ensures that running the pipeline twice does not duplicate front matter.
func TestPipeline_Idempotency(t *testing.T) {
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, t.TempDir())
	original := "---\ncustom: keep\n---\n# Heading\n\nLink to [Doc](doc.md)."
	file := docs.DocFile{Repository: "repo", Name: "page", RelativePath: "page.md", Content: []byte(original)}
	if err := gen.copyContentFiles(context.Background(), []docs.DocFile{file}); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	outPath := gen.buildRoot() + "/" + file.GetHugoPath(true)
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	first := string(data)
	if strings.Count(first, "---\n") != 2 {
		t.Fatalf("expected single front matter block, got %d", strings.Count(first, "---\n"))
	}
	if strings.Contains(first, "doc.md") {
		t.Fatalf("link not rewritten: %s", first)
	}
	// Re-run using transformed output as input
	file2 := docs.DocFile{Repository: "repo", Name: "page", RelativePath: "page.md", Content: data}
	if err := gen.copyContentFiles(context.Background(), []docs.DocFile{file2}); err != nil {
		t.Fatalf("second copy: %v", err)
	}
	data2, _ := os.ReadFile(outPath)
	if strings.Count(string(data2), "---\n") != 2 {
		t.Fatalf("idempotency failed; delimiters=%d", strings.Count(string(data2), "---\n"))
	}
}

// TestPipeline_Order verifies that front matter parsing happens before building.
func TestPipeline_Order(t *testing.T) {
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, t.TempDir())
	existing := "---\ncustom: val\n---\nBody"
	file := docs.DocFile{Repository: "r", Name: "body", RelativePath: "body.md", Content: []byte(existing)}
	if err := gen.copyContentFiles(context.Background(), []docs.DocFile{file}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	outPath := gen.buildRoot() + "/" + file.GetHugoPath(true)
	data, _ := os.ReadFile(outPath)
	out := string(data)
	if !strings.Contains(out, "custom: val") {
		t.Fatalf("existing key lost: %s", out)
	}
	if !strings.Contains(out, "repository: r") {
		t.Fatalf("repository missing: %s", out)
	}
}

// TestMalformedFrontMatter ensures invalid YAML front matter does not break build.
func TestMalformedFrontMatter(t *testing.T) {
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, t.TempDir())
	malformed := "---\n:bad yaml\n---\n# T\n"
	file := docs.DocFile{Repository: "r", Name: "bad", RelativePath: "bad.md", Content: []byte(malformed)}
	if err := gen.copyContentFiles(context.Background(), []docs.DocFile{file}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	data, _ := os.ReadFile(gen.buildRoot() + "/" + file.GetHugoPath(true))
	if !strings.Contains(string(data), "title:") {
		t.Fatalf("expected generated title, got %s", string(data))
	}
}

// TestDateConsistency ensures BuildFrontMatter uses Now injection indirectly through builder.
func TestDateConsistency(t *testing.T) {
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, t.TempDir())
	file := docs.DocFile{Repository: "repo", Name: "when", RelativePath: "when.md", Content: []byte("Body")}
	if err := gen.copyContentFiles(context.Background(), []docs.DocFile{file}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	data, _ := os.ReadFile(gen.buildRoot() + "/" + file.GetHugoPath(true))
	if !strings.Contains(string(data), "date:") {
		t.Fatalf("expected date in front matter, got %s", string(data))
	}
}
