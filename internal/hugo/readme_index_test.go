package hugo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

func TestReadmeAsRepositoryIndex(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Title = "Test Site"
	cfg.Hugo.Theme = "hextra"

	gen := hugo.NewGenerator(cfg, tmpDir)

	// Create a test README file
	readmeContent := "# My Repository\n\nThis is my custom README content."
	readmeFile := docs.DocFile{
		Repository:   "test-repo",
		Section:      "",
		Name:         "README",
		Extension:    ".md",
		RelativePath: "README.md",
		DocsBase:     "docs",
		Content:      []byte(readmeContent),
	}

	// Create a regular doc file
	regularFile := docs.DocFile{
		Repository:   "test-repo",
		Section:      "guides",
		Name:         "guide",
		Extension:    ".md",
		RelativePath: "guides/guide.md",
		DocsBase:     "docs",
		Content:      []byte("# Guide\n\nSome content"),
	}

	files := []docs.DocFile{readmeFile, regularFile}

	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("GenerateSite failed: %v", err)
	}

	// Check that README was used as repository index
	indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read repository index: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "This is my custom README content") {
		t.Errorf("Repository index should contain README content, got: %s", contentStr)
	}
}
