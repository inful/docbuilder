package editlink_test

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/editlink"
)

func TestVSCodeDetector_LocalPreview(t *testing.T) {
	detector := editlink.NewVSCodeDetector()

	// Test context for local preview mode
	ctx := editlink.DetectionContext{
		File: docs.DocFile{
			Repository:   "local",
			RelativePath: "README.md",
			DocsBase:     ".",
		},
		Repository: &config.Repository{
			Name: "local",
			URL:  "/workspaces/docbuilder/docs",
		},
		RepoRel: "README.md",
	}

	result := detector.Detect(ctx)

	if !result.Found {
		t.Fatal("Expected VS Code detector to find a match for local preview")
	}

	if result.ForgeType != "vscode" {
		t.Errorf("Expected forge type 'vscode', got '%s'", result.ForgeType)
	}

	if result.FullName != "README.md" {
		t.Errorf("Expected FullName 'README.md', got '%s'", result.FullName)
	}
}

func TestVSCodeDetector_NonLocalPreview(t *testing.T) {
	detector := editlink.NewVSCodeDetector()

	// Test context for normal repository (not local preview)
	ctx := editlink.DetectionContext{
		File: docs.DocFile{
			Repository:   "myrepo",
			RelativePath: "README.md",
			DocsBase:     "docs",
		},
		Repository: &config.Repository{
			Name: "myrepo",
			URL:  "https://github.com/org/repo.git",
		},
		RepoRel: "docs/README.md",
	}

	result := detector.Detect(ctx)

	if result.Found {
		t.Fatal("Expected VS Code detector to not match for non-local preview")
	}
}

func TestStandardEditURLBuilder_VSCode(t *testing.T) {
	builder := editlink.NewStandardEditURLBuilder()

	url := builder.BuildURL("vscode", "", "docs/README.md", "main", "docs/README.md")

	expectedURL := "/_edit/docs/README.md"
	if url != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, url)
	}
}
