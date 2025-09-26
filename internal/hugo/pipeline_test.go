package hugo

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestPipeline_Idempotency ensures that running the pipeline twice does not duplicate front matter.
func TestPipeline_Idempotency(t *testing.T) {
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, t.TempDir())

	original := "---\ncustom: keep\n---\n# Heading\n\nLink to [Doc](doc.md)."
	file := docs.DocFile{Repository: "repo", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte(original)}
	p := &Page{File: file, Raw: file.Content, Content: string(file.Content), FrontMatter: map[string]any{}}
	pipe := NewTransformerPipeline(&FrontMatterParser{}, &RelativeLinkRewriter{}, &FrontMatterBuilder{ConfigProvider: func() *Generator { return gen }}, &FinalFrontMatterSerializer{})
	if err := pipe.Run(p); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	firstOutput := string(p.Raw)
	if strings.Count(firstOutput, "---\n") < 1 {
		t.Fatalf("expected front matter once, got: %s", firstOutput)
	}
	if strings.Contains(firstOutput, "doc.md") {
		t.Fatalf("link rewrite failed: %s", firstOutput)
	}

	// Second run on produced output
	file2 := docs.DocFile{Repository: "repo", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte(firstOutput)}
	p2 := &Page{File: file2, Raw: file2.Content, Content: string(file2.Content), FrontMatter: map[string]any{}}
	if err := pipe.Run(p2); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	secondOutput := string(p2.Raw)

	// Expect exactly two delimiters: opening and closing front matter
	if strings.Count(secondOutput, "---\n") != 2 {
		t.Fatalf("unexpected front matter delimiter count after second run: %d output: %s", strings.Count(secondOutput, "---\n"), secondOutput)
	}
}

// TestPipeline_Order verifies that front matter parsing happens before building.
func TestPipeline_Order(t *testing.T) {
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, t.TempDir())
	existing := "---\ncustom: val\n---\nBody"
	file := docs.DocFile{Repository: "r", Name: "body", RelativePath: "body.md", Extension: ".md", Content: []byte(existing)}
	p := &Page{File: file, Raw: file.Content, Content: string(file.Content), FrontMatter: map[string]any{}}
	pipe := NewTransformerPipeline(&FrontMatterParser{}, &FrontMatterBuilder{ConfigProvider: func() *Generator { return gen }}, &FinalFrontMatterSerializer{})
	if err := pipe.Run(p); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	out := string(p.Raw)
	if !strings.Contains(out, "custom: val") {
		t.Fatalf("existing front matter key lost: %s", out)
	}
	if !strings.Contains(out, "repository: r") {
		t.Fatalf("builder did not add repository: %s", out)
	}
}

// TestMalformedFrontMatter ensures invalid YAML front matter does not break build.
func TestMalformedFrontMatter(t *testing.T) {
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, t.TempDir())
	malformed := "---\n:bad yaml\n---\n# T\n"
	file := docs.DocFile{Repository: "r", Name: "bad", RelativePath: "bad.md", Extension: ".md", Content: []byte(malformed)}
	p := &Page{File: file, Raw: file.Content, Content: string(file.Content), FrontMatter: map[string]any{}}
	pipe := NewTransformerPipeline(&FrontMatterParser{}, &FrontMatterBuilder{ConfigProvider: func() *Generator { return gen }}, &FinalFrontMatterSerializer{})
	if err := pipe.Run(p); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	out := string(p.Raw)
	if !strings.Contains(out, "title:") {
		t.Fatalf("expected generated front matter title, got: %s", out)
	}
}

// TestDateConsistency ensures BuildFrontMatter uses Now injection indirectly through builder.
func TestDateConsistency(t *testing.T) {
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, t.TempDir())
	file := docs.DocFile{Repository: "repo", Name: "when", RelativePath: "when.md", Extension: ".md", Content: []byte("Body")}
	p := &Page{File: file, Raw: file.Content, Content: string(file.Content), FrontMatter: map[string]any{}}
	pipe := NewTransformerPipeline(&FrontMatterBuilder{ConfigProvider: func() *Generator { return gen }}, &FinalFrontMatterSerializer{})
	if err := pipe.Run(p); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	if !strings.Contains(string(p.Raw), "date:") {
		t.Fatalf("expected date in serialized output, got %s", string(p.Raw))
	}
}
