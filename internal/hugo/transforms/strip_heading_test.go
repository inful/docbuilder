package transforms

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestStripFirstHeading_PreservesHeadingForIndexFiles(t *testing.T) {
	tests := []struct {
		name           string
		fileName       string
		inputContent   string
		expectedOutput string
		shouldStrip    bool
	}{
		{
			name:           "index.md should strip H1 when content starts with H1",
			fileName:       "index",
			inputContent:   "# This is a custom index\n\nSome content here.",
			expectedOutput: "Some content here.",
			shouldStrip:    true,
		},
		{
			name:           "README.md should strip H1 when content starts with H1",
			fileName:       "README",
			inputContent:   "# Repository README\n\nWelcome text.",
			expectedOutput: "Welcome text.",
			shouldStrip:    true,
		},
		{
			name:           "README.md should preserve H1 when text exists before it",
			fileName:       "README",
			inputContent:   "Some intro text.\n\n# Repository README\n\nWelcome text.",
			expectedOutput: "Some intro text.\n\n# Repository README\n\nWelcome text.",
			shouldStrip:    false,
		},
		{
			name:           "regular file should strip H1",
			fileName:       "guide",
			inputContent:   "# Getting Started\n\nFollow these steps.",
			expectedOutput: "Follow these steps.",
			shouldStrip:    true,
		},
		{
			name:           "api-reference should strip H1",
			fileName:       "api-reference",
			inputContent:   "# API Reference\n\nAPI docs here.",
			expectedOutput: "API docs here.",
			shouldStrip:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a PageShim with the test content
			shim := &PageShim{
				Doc: docs.DocFile{
					Name: tt.fileName,
				},
				Content: tt.inputContent,
			}

			// Apply the transform
			transform := stripFirstHeadingTransform{}
			err := transform.Transform(shim)
			if err != nil {
				t.Fatalf("transform failed: %v", err)
			}

			// Verify the result
			if shim.Content != tt.expectedOutput {
				t.Errorf("Content mismatch:\n  got: %q\n  want: %q", shim.Content, tt.expectedOutput)
			}

			// Additional verification
			hasH1After := len(shim.Content) > 0 && shim.Content[0] == '#'
			if tt.shouldStrip && hasH1After {
				t.Error("Expected H1 to be stripped but it's still present")
			}
			if !tt.shouldStrip && !hasH1After && len(tt.inputContent) > 0 && tt.inputContent[0] == '#' {
				t.Error("Expected H1 to be preserved but it was stripped")
			}
		})
	}
}
