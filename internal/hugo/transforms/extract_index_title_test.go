package transforms

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

func TestExtractIndexTitle(t *testing.T) {
	tests := []struct {
		name               string
		fileName           string
		section            string
		content            string
		existingFrontMatter map[string]any
		expectedTitle      string
		shouldAddPatch     bool
	}{
		{
			name:               "index.md with H1",
			fileName:           "index",
			content:            "# Custom Index Title\n\nSome content here.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "Custom Index Title",
			shouldAddPatch:     true,
		},
		{
			name:               "README.md with H1",
			fileName:           "README",
			content:            "# Repository Overview\n\nWelcome text.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "Repository Overview",
			shouldAddPatch:     true,
		},
		{
			name:               "index.md with existing title should not override",
			fileName:           "index",
			content:            "# Content Heading\n\nSome content.",
			existingFrontMatter: map[string]any{"title": "Explicit Title"},
			expectedTitle:      "",
			shouldAddPatch:     false,
		},
		{
			name:               "regular file should not be processed",
			fileName:           "guide",
			content:            "# Getting Started\n\nFollow these steps.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "",
			shouldAddPatch:     false,
		},
		{
			name:               "index.md without H1",
			fileName:           "index",
			content:            "Just plain content without heading.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "",
			shouldAddPatch:     false,
		},
		{
			name:               "index.md with H1 having extra whitespace",
			fileName:           "index",
			content:            "  #   Trimmed Title   \n\nContent.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "Trimmed Title",
			shouldAddPatch:     true,
		},
		{
			name:               "index.md with section uses section name as title",
			fileName:           "index",
			content:            "# Docs\n\nSome content.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "Vcfretriever",
			shouldAddPatch:     true,
			section:            "vcfretriever",
		},
		{
			name:               "README.md with section uses section name as title",
			fileName:           "README",
			content:            "# Old Title\n\nWelcome text.",
			existingFrontMatter: map[string]any{},
			expectedTitle:      "My Project",
			shouldAddPatch:     true,
			section:            "my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a PageShim with the test data
			shim := &PageShim{
				Doc: docs.DocFile{
					Name:    tt.fileName,
					Section: tt.section,
				},
				Content:             tt.content,
				OriginalFrontMatter: tt.existingFrontMatter,
				Patches:             []fmcore.FrontMatterPatch{},
			}

			// Set up the AddPatch function to capture patches
			var capturedPatches []fmcore.FrontMatterPatch
			shim.BackingAddPatch = func(patch fmcore.FrontMatterPatch) {
				capturedPatches = append(capturedPatches, patch)
			}

			// Apply the transform
			transform := extractIndexTitleTransform{}
			err := transform.Transform(shim)
			if err != nil {
				t.Fatalf("transform failed: %v", err)
			}

			// Verify patch was added or not as expected
			patchAdded := len(capturedPatches) > 0
			if patchAdded != tt.shouldAddPatch {
				t.Errorf("Expected shouldAddPatch=%v, but patch was added=%v", tt.shouldAddPatch, patchAdded)
			}

			// If a patch was expected, verify the title
			if tt.shouldAddPatch && patchAdded {
				patch := capturedPatches[0]
				if title, ok := patch.Data["title"].(string); ok {
					if title != tt.expectedTitle {
						t.Errorf("Expected title %q, got %q", tt.expectedTitle, title)
					}
				} else {
					t.Error("Patch does not contain a title field or it's not a string")
				}

				// Verify patch metadata
				if patch.Source != "extract_index_title" {
					t.Errorf("Expected patch source 'extract_index_title', got %q", patch.Source)
				}
				if patch.Mode != fmcore.MergeDeep {
					t.Errorf("Expected patch mode MergeDeep, got %v", patch.Mode)
				}
			}
		})
	}
}
