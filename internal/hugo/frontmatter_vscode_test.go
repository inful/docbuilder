package hugo_test

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// TestBuildFrontMatter_VSCodeEditURL verifies that edit URLs are generated for local preview mode.
func TestBuildFrontMatter_VSCodeEditURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Repositories = []config.Repository{
		{
			Name:   "local",
			URL:    "/workspaces/docbuilder/docs",
			Branch: "",
			Paths:  []string{"."},
		},
	}

	file := docs.DocFile{
		Repository:   "local",
		RelativePath: "README.md",
		DocsBase:     ".",
		Name:         "README",
	}

	in := hugo.FrontMatterInput{
		File:     file,
		Existing: make(map[string]any),
		Config:   cfg,
		Now:      time.Now(),
	}

	fm := hugo.BuildFrontMatter(in)

	editURL, exists := fm["editURL"]
	if !exists {
		t.Fatal("Expected editURL to be present in frontmatter for local preview mode")
	}

	expectedURL := "/_edit/README.md"
	if editURL != expectedURL {
		t.Errorf("Expected editURL '%s', got '%v'", expectedURL, editURL)
	}
}
