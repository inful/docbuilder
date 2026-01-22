package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// TestUseReadmeAsIndex_WithExistingFrontMatter tests README with valid front matter.
func TestUseReadmeAsIndex_WithExistingFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	readmeContent := `---
title: "Test Repository"
---

# Test Repo

This is a test repository.
`

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte(readmeContent),
	}

	indexPath := filepath.Join(tmpDir, "content", "test", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	// #nosec G304 -- test utility reading from test output directory
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index file: %v", err)
	}

	// Verify front matter has required fields
	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Fatal("index file is empty")
	}

	// Parse front matter to verify fields
	fmRaw, _, had, _, err := frontmatter.Split([]byte(contentStr))
	if err != nil {
		t.Fatalf("failed to split front matter: %v", err)
	}
	if !had {
		t.Fatal("no front matter found")
	}
	fm, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		t.Fatalf("failed to parse front matter: %v", err)
	}

	// Check required fields were added
	if fm["type"] == nil {
		t.Error("expected type field to be set")
	}
	if fm["repository"] == nil {
		t.Error("expected repository field to be set")
	}
	if fm["date"] == nil {
		t.Error("expected date field to be set")
	}
	if fm["title"] == nil {
		t.Error("expected title field to be preserved")
	}
}

// TestUseReadmeAsIndex_WithoutFrontMatter tests README without front matter.
func TestUseReadmeAsIndex_WithoutFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	readmeContent := `# Test Repository

This is a test repository without front matter.
`

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte(readmeContent),
	}

	indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	// #nosec G304 -- test utility reading from test output directory
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index file: %v", err)
	}

	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Fatal("index file is empty")
	}

	// Parse front matter
	fmRaw, body, had, _, err := frontmatter.Split([]byte(contentStr))
	if err != nil {
		t.Fatalf("failed to split front matter: %v", err)
	}
	if !had {
		t.Fatal("no front matter found")
	}
	fm, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		t.Fatalf("failed to parse front matter: %v", err)
	}

	// Check all required fields were added
	if fm["title"] == nil {
		t.Error("expected title field to be added")
	}
	if fm["repository"] != "test-repo" {
		t.Errorf("expected repository='test-repo', got %v", fm["repository"])
	}
	if fm["type"] != "docs" {
		t.Errorf("expected type='docs', got %v", fm["type"])
	}
	if fm["date"] == nil {
		t.Error("expected date field to be added")
	}

	// Verify content is preserved
	if len(body) == 0 {
		t.Error("expected content body to be preserved")
	}
}

// TestUseReadmeAsIndex_EmptyTransformedBytes tests error handling for empty content.
func TestUseReadmeAsIndex_EmptyTransformedBytes(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte{}, // Empty
	}

	indexPath := filepath.Join(tmpDir, "content", "test", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err == nil {
		t.Fatal("expected error for empty transformed bytes")
	}
}

// TestUseReadmeAsIndex_InvalidYAML tests handling of malformed front matter.
func TestUseReadmeAsIndex_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	readmeContent := `---
title: "Unclosed quote
invalid: [yaml
---

# Test
`

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte(readmeContent),
	}

	indexPath := filepath.Join(tmpDir, "content", "test", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// TestUseReadmeAsIndex_MalformedFrontMatter_TreatedAsAbsent ensures unterminated front matter
// does not fail index generation (treated as absent, like legacy behavior).
func TestUseReadmeAsIndex_MalformedFrontMatter_TreatedAsAbsent(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	// Missing closing front matter delimiter
	readmeContent := "---\ntitle: test\n\n# Body\n"

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte(readmeContent),
	}

	indexPath := filepath.Join(tmpDir, "content", "test", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err != nil {
		t.Fatalf("unexpected error for malformed front matter: %v", err)
	}

	// #nosec G304 -- test utility reading from test output directory
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index file: %v", err)
	}
	if !bytes.Contains(content, []byte("title:")) {
		t.Fatalf("expected generated front matter to include title, got: %s", string(content))
	}
}

// TestUseReadmeAsIndex_PartialFrontMatter tests front matter with some fields missing.
func TestUseReadmeAsIndex_PartialFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	// Front matter missing type and repository
	readmeContent := `---
title: "Partial Front Matter"
---

# Content
`

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte(readmeContent),
	}

	indexPath := filepath.Join(tmpDir, "content", "test", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	// #nosec G304 -- test utility reading from test output directory
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index file: %v", err)
	}

	// Parse front matter
	fmRaw, _, had, _, err := frontmatter.Split(content)
	if err != nil {
		t.Fatalf("failed to split front matter: %v", err)
	}
	if !had {
		t.Fatal("no front matter found")
	}
	fm, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		t.Fatalf("failed to parse front matter: %v", err)
	}

	// Check required fields were added while preserving existing
	if fm["title"] == nil {
		t.Error("expected title to be preserved")
	}
	if fm["type"] == nil {
		t.Error("expected type field to be added")
	}
	if fm["repository"] == nil {
		t.Error("expected repository field to be added")
	}
	if fm["date"] == nil {
		t.Error("expected date field to be added")
	}
}

// TestUseReadmeAsIndex_FrontMatterWithAllFields tests when all fields are already present.
func TestUseReadmeAsIndex_FrontMatterWithAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{
		config:    &config.Config{},
		outputDir: tmpDir,
	}

	readmeContent := `---
title: "Complete Front Matter"
type: "custom"
repository: "existing-repo"
date: "2023-12-01T00:00:00Z"
---

# Content
`

	readmeFile := &docs.DocFile{
		Path:             "/test/README.md",
		RelativePath:     "test/README.md",
		TransformedBytes: []byte(readmeContent),
	}

	indexPath := filepath.Join(tmpDir, "content", "test", "_index.md")
	err := g.useReadmeAsIndex(readmeFile, indexPath, "test-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	// #nosec G304 -- test utility reading from test output directory
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index file: %v", err)
	}

	// Parse front matter
	fmRaw, _, had, _, err := frontmatter.Split(content)
	if err != nil {
		t.Fatalf("failed to split front matter: %v", err)
	}
	if !had {
		t.Fatal("no front matter found")
	}
	fm, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		t.Fatalf("failed to parse front matter: %v", err)
	}

	// Verify existing values were preserved (not overwritten)
	if fm["type"] != "custom" {
		t.Errorf("expected type='custom', got %v", fm["type"])
	}
	if fm["repository"] != "existing-repo" {
		t.Errorf("expected repository='existing-repo', got %v", fm["repository"])
	}
}
