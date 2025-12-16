package transforms

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// TestExtractIndexTitle_RealWorld tests with content matching testdocs/README.md
func TestExtractIndexTitle_RealWorld(t *testing.T) {
	// Simulate what front_matter_parser would produce: stripped front matter, content only
	content := `# A Readme

This is a readme - it should be included`

	originalFM := map[string]any{
		"tags":       []any{"documentation", "readme"},
		"categories": []any{"getting-started"},
	}

	pg := &PageShim{
		Doc: docs.DocFile{
			Name:    "README",
			Section: "", // root level
		},
		Content:             content,
		OriginalFrontMatter: originalFM,
		Patches:             []fmcore.FrontMatterPatch{},
	}

	transform := extractIndexTitleTransform{}
	err := transform.Transform(pg)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Should have added a patch with title "A Readme"
	if len(pg.Patches) != 1 {
		t.Fatalf("Expected 1 patch, got %d", len(pg.Patches))
	}

	patch := pg.Patches[0]
	if patch.Source != "extract_index_title" {
		t.Errorf("Expected source 'extract_index_title', got '%s'", patch.Source)
	}

	if patch.Priority != 55 {
		t.Errorf("Expected priority 55, got %d", patch.Priority)
	}

	title, ok := patch.Data["title"]
	if !ok {
		t.Fatal("Expected title in patch data")
	}

	if title != "A Readme" {
		t.Errorf("Expected title 'A Readme', got '%s'", title)
	}
}
