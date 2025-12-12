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

func TestIndexMdAsRepositoryIndex(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Title = "Test Site"
	cfg.Hugo.Theme = "hextra"

	gen := hugo.NewGenerator(cfg, tmpDir)

	// Create a test index.md file (not README)
	indexContent := "# Custom Repository Index\n\nThis is my custom index.md at repository level."
	indexFile := docs.DocFile{
		Repository:   "test-repo",
		Section:      "",
		Name:         "index",
		Extension:    ".md",
		RelativePath: "index.md",
		DocsBase:     "docs",
		Content:      []byte(indexContent),
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

	files := []docs.DocFile{indexFile, regularFile}

	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("GenerateSite failed: %v", err)
	}

	// Check that index.md was used as repository index
	indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read repository index: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "This is my custom index.md at repository level") {
		t.Errorf("Repository index should contain index.md content, got: %s", contentStr)
	}
}

func TestRepositoryIndexPrecedence(t *testing.T) {
	t.Run("Case1_ReadmeOnly", func(t *testing.T) {
		// Case 1: Only README.md exists → should be used as _index.md
		tmpDir := t.TempDir()
		cfg := &config.Config{}
		cfg.Hugo.Title = "Test Site"
		cfg.Hugo.Theme = "hextra"
		gen := hugo.NewGenerator(cfg, tmpDir)

		files := []docs.DocFile{
			{
				Repository:   "test-repo",
				Section:      "",
				Name:         "README",
				Extension:    ".md",
				RelativePath: "README.md",
				DocsBase:     "docs",
				Content:      []byte("# Repository README\n\nREADME content."),
			},
			{
				Repository:   "test-repo",
				Section:      "guides",
				Name:         "document",
				Extension:    ".md",
				RelativePath: "guides/document.md",
				DocsBase:     "docs",
				Content:      []byte("# Guide\n\nSome content"),
			},
		}

		if err := gen.GenerateSite(files); err != nil {
			t.Fatalf("GenerateSite failed: %v", err)
		}

		indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
		content, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("Failed to read repository index: %v", err)
		}

		if !strings.Contains(string(content), "README content") {
			t.Errorf("Case 1: Repository index should use README.md, got: %s", string(content))
		}
	})

	t.Run("Case2_NoUserIndex", func(t *testing.T) {
		// Case 2: No README or index.md → should auto-generate
		tmpDir := t.TempDir()
		cfg := &config.Config{}
		cfg.Hugo.Title = "Test Site"
		cfg.Hugo.Theme = "hextra"
		gen := hugo.NewGenerator(cfg, tmpDir)

		files := []docs.DocFile{
			{
				Repository:   "test-repo",
				Section:      "guides",
				Name:         "document",
				Extension:    ".md",
				RelativePath: "guides/document.md",
				DocsBase:     "docs",
				Content:      []byte("# Guide\n\nSome content"),
			},
		}

		if err := gen.GenerateSite(files); err != nil {
			t.Fatalf("GenerateSite failed: %v", err)
		}

		indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
		content, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("Failed to read repository index: %v", err)
		}

		// Should contain auto-generated content (title, sections list)
		if !strings.Contains(string(content), "title:") {
			t.Errorf("Case 2: Repository index should be auto-generated, got: %s", string(content))
		}
	})

	t.Run("Case3_IndexMdOnly", func(t *testing.T) {
		// Case 3: Only index.md exists → should be used as _index.md
		tmpDir := t.TempDir()
		cfg := &config.Config{}
		cfg.Hugo.Title = "Test Site"
		cfg.Hugo.Theme = "hextra"
		gen := hugo.NewGenerator(cfg, tmpDir)

		files := []docs.DocFile{
			{
				Repository:   "test-repo",
				Section:      "",
				Name:         "index",
				Extension:    ".md",
				RelativePath: "index.md",
				DocsBase:     "docs",
				Content:      []byte("# Custom Index\n\nindex.md content."),
			},
			{
				Repository:   "test-repo",
				Section:      "guides",
				Name:         "document",
				Extension:    ".md",
				RelativePath: "guides/document.md",
				DocsBase:     "docs",
				Content:      []byte("# Guide\n\nSome content"),
			},
		}

		if err := gen.GenerateSite(files); err != nil {
			t.Fatalf("GenerateSite failed: %v", err)
		}

		indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
		content, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("Failed to read repository index: %v", err)
		}

		if !strings.Contains(string(content), "index.md content") {
			t.Errorf("Case 3: Repository index should use index.md, got: %s", string(content))
		}
	})

	t.Run("Case4_BothIndexAndReadme", func(t *testing.T) {
		// Case 4: Both README.md and index.md exist → index.md takes precedence
		// README.md should be copied as a regular document
		tmpDir := t.TempDir()
		cfg := &config.Config{}
		cfg.Hugo.Title = "Test Site"
		cfg.Hugo.Theme = "hextra"
		gen := hugo.NewGenerator(cfg, tmpDir)

		files := []docs.DocFile{
			{
				Repository:   "test-repo",
				Section:      "",
				Name:         "README",
				Extension:    ".md",
				RelativePath: "README.md",
				DocsBase:     "docs",
				Content:      []byte("# Repository README\n\nREADME content."),
			},
			{
				Repository:   "test-repo",
				Section:      "",
				Name:         "index",
				Extension:    ".md",
				RelativePath: "index.md",
				DocsBase:     "docs",
				Content:      []byte("# Custom Index\n\nindex.md content (should win)."),
			},
			{
				Repository:   "test-repo",
				Section:      "guides",
				Name:         "document",
				Extension:    ".md",
				RelativePath: "guides/document.md",
				DocsBase:     "docs",
				Content:      []byte("# Guide\n\nSome content"),
			},
		}

		if err := gen.GenerateSite(files); err != nil {
			t.Fatalf("GenerateSite failed: %v", err)
		}

		// Check repository index uses index.md
		indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
		content, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("Failed to read repository index: %v", err)
		}

		if !strings.Contains(string(content), "index.md content (should win)") {
			t.Errorf("Case 4: Repository index should use index.md (precedence), got: %s", string(content))
		}

		if strings.Contains(string(content), "README content") {
			t.Errorf("Case 4: Repository index should NOT contain README content, got: %s", string(content))
		}

		// Check that README.md was copied as a regular document (lowercase due to GetHugoPath)
		readmePath := filepath.Join(tmpDir, "content", "test-repo", "readme.md")
		readmeContent, err := os.ReadFile(readmePath)
		if err != nil {
			t.Fatalf("Failed to read readme.md as document: %v", err)
		}

		if !strings.Contains(string(readmeContent), "README content") {
			t.Errorf("Case 4: README.md should be copied as regular document, got: %s", string(readmeContent))
		}
	})
}
