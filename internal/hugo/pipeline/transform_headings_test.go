package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractIndexTitle_Idempotent verifies that title extraction is idempotent.
func TestExtractIndexTitle_Idempotent(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "index with H1 at start",
			doc: &Document{
				Content:     "# Welcome\n\nSome content here.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
		},
		{
			name: "index with H1 after text",
			doc: &Document{
				Content:     "Some intro text\n\n# Welcome\n\nContent.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
		},
		{
			name: "index without H1",
			doc: &Document{
				Content:     "Just some content without heading.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
		},
		{
			name: "non-index file",
			doc: &Document{
				Content:     "# Title\n\nContent.",
				FrontMatter: make(map[string]any),
				IsIndex:     false,
			},
		},
		{
			name: "index with existing title",
			doc: &Document{
				Content: "# Welcome\n\nSome content.",
				FrontMatter: map[string]any{
					"title": "Existing Title",
				},
				IsIndex: true,
			},
		},
		{
			name: "index with H1 and whitespace",
			doc: &Document{
				Content:     "   \n\n# Welcome Guide\n\nContent here.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create copies for multiple runs
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := extractIndexTitle(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture state after first application
			state1 := captureDocumentState(doc1)

			// Apply transform second time
			newDocs2, err2 := extractIndexTitle(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture state after second application
			state2 := captureDocumentState(doc1)

			// States should be identical (idempotent)
			assert.Equal(t, state1, state2, "applying transform twice should produce same result")

			// Also verify independent application produces same result
			newDocs3, err3 := extractIndexTitle(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			state3 := captureDocumentState(doc2)
			assert.Equal(t, state1, state3, "independent application should produce same result")
		})
	}
}

// TestStripHeading_Idempotent verifies that heading stripping is idempotent.
func TestStripHeading_Idempotent(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "H1 matching title",
			doc: &Document{
				Content: "# Welcome\n\nSome content here.",
				FrontMatter: map[string]any{
					"title": "Welcome",
				},
			},
		},
		{
			name: "H1 not matching title",
			doc: &Document{
				Content: "# Different Heading\n\nContent.",
				FrontMatter: map[string]any{
					"title": "Welcome",
				},
			},
		},
		{
			name: "no H1",
			doc: &Document{
				Content: "Just content without heading.",
				FrontMatter: map[string]any{
					"title": "Welcome",
				},
			},
		},
		{
			name: "no title in frontmatter",
			doc: &Document{
				Content:     "# Some Heading\n\nContent.",
				FrontMatter: make(map[string]any),
			},
		},
		{
			name: "H1 with extra whitespace matching title",
			doc: &Document{
				Content: "#   Welcome   \n\nContent.",
				FrontMatter: map[string]any{
					"title": "  Welcome  ",
				},
			},
		},
		{
			name: "H1 with newline after",
			doc: &Document{
				Content: "# Welcome\n\nParagraph here.",
				FrontMatter: map[string]any{
					"title": "Welcome",
				},
			},
		},
		{
			name: "H1 without newline after",
			doc: &Document{
				Content: "# Welcome\nImmediately following text.",
				FrontMatter: map[string]any{
					"title": "Welcome",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create copies for multiple runs
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := stripHeading(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture state after first application
			state1 := captureDocumentState(doc1)

			// Apply transform second time
			newDocs2, err2 := stripHeading(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture state after second application
			state2 := captureDocumentState(doc1)

			// States should be identical (idempotent)
			assert.Equal(t, state1, state2, "applying transform twice should produce same result")

			// Also verify independent application produces same result
			newDocs3, err3 := stripHeading(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			state3 := captureDocumentState(doc2)
			assert.Equal(t, state1, state3, "independent application should produce same result")
		})
	}
}

// TestExtractAndStripCombined_Idempotent verifies idempotence of the combined workflow.
func TestExtractAndStripCombined_Idempotent(t *testing.T) {
	tests := []struct {
		name            string
		initialDoc      *Document
		expectedTitle   string
		expectedContent string
	}{
		{
			name: "extract and strip matching H1",
			initialDoc: &Document{
				Content:     "# Welcome Guide\n\nSome content here.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
			expectedTitle:   "Welcome Guide",
			expectedContent: "\nSome content here.",
		},
		{
			name: "extract but don't strip - text before H1",
			initialDoc: &Document{
				Content:     "Intro text.\n\n# Welcome\n\nContent.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
			expectedTitle:   "", // No title extracted due to text before H1
			expectedContent: "Intro text.\n\n# Welcome\n\nContent.",
		},
		{
			name: "no H1 to extract or strip",
			initialDoc: &Document{
				Content:     "Just content.",
				FrontMatter: make(map[string]any),
				IsIndex:     true,
			},
			expectedTitle:   "",
			expectedContent: "Just content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First run: extract then strip
			doc1 := cloneDocument(tt.initialDoc)
			_, err := extractIndexTitle(doc1)
			require.NoError(t, err)
			_, err = stripHeading(doc1)
			require.NoError(t, err)

			state1 := captureDocumentState(doc1)

			// Second run: extract then strip again
			_, err = extractIndexTitle(doc1)
			require.NoError(t, err)
			_, err = stripHeading(doc1)
			require.NoError(t, err)

			state2 := captureDocumentState(doc1)

			// Should be identical
			assert.Equal(t, state1, state2, "combined workflow should be idempotent")

			// Verify expected results
			if tt.expectedTitle != "" {
				assert.Equal(t, tt.expectedTitle, doc1.FrontMatter["title"])
			} else {
				_, hasTitle := doc1.FrontMatter["title"]
				assert.False(t, hasTitle, "should not have title")
			}
			assert.Equal(t, tt.expectedContent, doc1.Content)
		})
	}
}

// TestStripHeading_BehaviorVerification tests specific edge cases.
func TestStripHeading_BehaviorVerification(t *testing.T) {
	t.Run("strips H1 only when exact match", func(t *testing.T) {
		doc := &Document{
			Content: "# Welcome\n\nContent here.",
			FrontMatter: map[string]any{
				"title": "Welcome",
			},
		}

		_, err := stripHeading(doc)
		require.NoError(t, err)

		assert.Equal(t, "\nContent here.", doc.Content)
		assert.NotContains(t, doc.Content, "# Welcome")
	})

	t.Run("preserves H1 when no match", func(t *testing.T) {
		doc := &Document{
			Content: "# Different\n\nContent here.",
			FrontMatter: map[string]any{
				"title": "Welcome",
			},
		}

		_, err := stripHeading(doc)
		require.NoError(t, err)

		assert.Equal(t, "# Different\n\nContent here.", doc.Content)
		assert.Contains(t, doc.Content, "# Different")
	})

	t.Run("handles whitespace in comparison", func(t *testing.T) {
		doc := &Document{
			Content: "#   Welcome   \n\nContent.",
			FrontMatter: map[string]any{
				"title": "  Welcome  ",
			},
		}

		_, err := stripHeading(doc)
		require.NoError(t, err)

		// Should strip because trimmed values match
		assert.NotContains(t, doc.Content, "# Welcome")
	})

	t.Run("strips H1 when it starts with title (partial match)", func(t *testing.T) {
		doc := &Document{
			Content: "# ADR-000: Uniform Error Handling Across DocBuilder\n\nContent here.",
			FrontMatter: map[string]any{
				"title": "ADR-000: Uniform Error Handling",
			},
		}

		_, err := stripHeading(doc)
		require.NoError(t, err)

		// Should strip because H1 starts with front matter title
		assert.NotContains(t, doc.Content, "# ADR-000")
		assert.Contains(t, doc.Content, "Content here.")
	})

	t.Run("case-insensitive partial match", func(t *testing.T) {
		doc := &Document{
			Content: "# Getting Started with DocBuilder\n\nContent.",
			FrontMatter: map[string]any{
				"title": "Getting Started",
			},
		}

		_, err := stripHeading(doc)
		require.NoError(t, err)

		// Should strip because H1 starts with title (case-insensitive)
		assert.NotContains(t, doc.Content, "# Getting Started")
	})
}
