package pipeline

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestAddRepositoryMetadata_Idempotent verifies that applying the transform twice
// produces the same result as applying it once.
func TestAddRepositoryMetadata_Idempotent(t *testing.T) {
	cfg := &config.Config{}
	transform := addRepositoryMetadata(cfg)

	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "all fields present",
			doc: &Document{
				FrontMatter:  make(map[string]any),
				Repository:   "test-repo",
				Forge:        "github",
				SourceCommit: "abc123def456",
			},
		},
		{
			name: "only repository",
			doc: &Document{
				FrontMatter: make(map[string]any),
				Repository:  "test-repo",
			},
		},
		{
			name: "empty repository",
			doc: &Document{
				FrontMatter: make(map[string]any),
			},
		},
		{
			name: "existing frontmatter",
			doc: &Document{
				FrontMatter: map[string]any{
					"title":        "Test Title",
					"repository":   "old-repo",
					"custom_field": "custom_value",
				},
				Repository:   "new-repo",
				Forge:        "gitlab",
				SourceCommit: "xyz789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy for the second run
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := transform(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture state after first application
			state1 := captureDocumentState(doc1)

			// Apply transform second time
			newDocs2, err2 := transform(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture state after second application
			state2 := captureDocumentState(doc1)

			// States should be identical (idempotent)
			assert.Equal(t, state1, state2, "applying transform twice should produce same result")

			// Also verify independent application produces same result
			newDocs3, err3 := transform(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			state3 := captureDocumentState(doc2)
			assert.Equal(t, state1, state3, "independent application should produce same result")
		})
	}
}

// TestAddEditLink_Idempotent verifies that edit link generation is idempotent.
func TestAddEditLink_Idempotent(t *testing.T) {
	cfg := &config.Config{}
	transform := addEditLink(cfg)

	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "github repository",
			doc: &Document{
				FrontMatter:  make(map[string]any),
				Repository:   "test-repo",
				Forge:        "github",
				SourceURL:    "https://github.com/org/repo.git",
				SourceBranch: "main",
				RelativePath: "docs/guide.md",
				DocsBase:     "docs",
				Generated:    false,
			},
		},
		{
			name: "gitlab repository",
			doc: &Document{
				FrontMatter:  make(map[string]any),
				Repository:   "test-repo",
				Forge:        "gitlab",
				SourceURL:    "https://gitlab.com/org/repo.git",
				SourceBranch: "develop",
				RelativePath: "api/reference.md",
				DocsBase:     "api",
				Generated:    false,
			},
		},
		{
			name: "forgejo repository",
			doc: &Document{
				FrontMatter:  make(map[string]any),
				Repository:   "test-repo",
				Forge:        "forgejo",
				SourceURL:    "https://git.example.com/user/project",
				SourceBranch: "main",
				RelativePath: "README.md",
				DocsBase:     "",
				Generated:    false,
			},
		},
		{
			name: "existing editURL",
			doc: &Document{
				FrontMatter: map[string]any{
					"editURL": "https://example.com/custom/edit/path",
				},
				Repository:   "test-repo",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "docs/guide.md",
				Generated:    false,
			},
		},
		{
			name: "generated document",
			doc: &Document{
				FrontMatter:  make(map[string]any),
				Repository:   "test-repo",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "_index.md",
				Generated:    true,
			},
		},
		{
			name: "missing source info",
			doc: &Document{
				FrontMatter: make(map[string]any),
				Repository:  "test-repo",
				Generated:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy for the second run
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := transform(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture state after first application
			state1 := captureDocumentState(doc1)

			// Apply transform second time
			newDocs2, err2 := transform(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture state after second application
			state2 := captureDocumentState(doc1)

			// States should be identical (idempotent)
			assert.Equal(t, state1, state2, "applying transform twice should produce same result")

			// Also verify independent application produces same result
			newDocs3, err3 := transform(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			state3 := captureDocumentState(doc2)
			assert.Equal(t, state1, state3, "independent application should produce same result")
		})
	}
}

// TestEditLinkGeneration_NoOverwrite verifies that existing edit URLs are preserved.
func TestEditLinkGeneration_NoOverwrite(t *testing.T) {
	cfg := &config.Config{}
	transform := addEditLink(cfg)

	customURL := "https://custom.example.com/edit"
	doc := &Document{
		FrontMatter: map[string]any{
			"editURL": customURL,
		},
		Repository:   "test-repo",
		SourceURL:    "https://github.com/org/repo",
		SourceBranch: "main",
		RelativePath: "docs/guide.md",
		DocsBase:     "docs",
		Generated:    false,
	}

	// Apply transform
	newDocs, err := transform(doc)
	require.NoError(t, err)
	assert.Nil(t, newDocs)

	// Verify custom URL was preserved
	assert.Equal(t, customURL, doc.FrontMatter["editURL"], "existing editURL should not be overwritten")
}

// TestEditLinkGeneration_GeneratedDocuments verifies generated docs don't get edit links.
func TestEditLinkGeneration_GeneratedDocuments(t *testing.T) {
	cfg := &config.Config{}
	transform := addEditLink(cfg)

	doc := &Document{
		FrontMatter:  make(map[string]any),
		Repository:   "test-repo",
		SourceURL:    "https://github.com/org/repo",
		SourceBranch: "main",
		RelativePath: "_index.md",
		DocsBase:     "docs",
		Generated:    true, // Generated document
	}

	// Apply transform
	newDocs, err := transform(doc)
	require.NoError(t, err)
	assert.Nil(t, newDocs)

	// Verify no editURL was added
	_, exists := doc.FrontMatter["editURL"]
	assert.False(t, exists, "generated documents should not get editURL")
}

// TestGenerateEditURL_ForgeTypes verifies correct URL patterns for different forges.
func TestGenerateEditURL_ForgeTypes(t *testing.T) {
	tests := []struct {
		name        string
		doc         *Document
		expectedURL string
	}{
		{
			name: "github with .git suffix",
			doc: &Document{
				Forge:        "github",
				SourceURL:    "https://github.com/org/repo.git",
				SourceBranch: "main",
				RelativePath: "guide.md",
				DocsBase:     "docs",
			},
			expectedURL: "https://github.com/org/repo/edit/main/docs/guide.md",
		},
		{
			name: "github without .git suffix",
			doc: &Document{
				Forge:        "github",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "develop",
				RelativePath: "api.md",
				DocsBase:     "",
			},
			expectedURL: "https://github.com/org/repo/edit/develop/api.md",
		},
		{
			name: "gitlab",
			doc: &Document{
				Forge:        "gitlab",
				SourceURL:    "https://gitlab.com/group/project.git",
				SourceBranch: "main",
				RelativePath: "reference.md",
				DocsBase:     "documentation",
			},
			expectedURL: "https://gitlab.com/group/project/-/edit/main/documentation/reference.md",
		},
		{
			name: "forgejo",
			doc: &Document{
				Forge:        "forgejo",
				SourceURL:    "https://git.example.com/user/repo",
				SourceBranch: "master",
				RelativePath: "README.md",
				DocsBase:     "",
			},
			expectedURL: "https://git.example.com/user/repo/_edit/master/README.md",
		},
		{
			name: "gitea",
			doc: &Document{
				Forge:        "gitea",
				SourceURL:    "https://gitea.example.com/org/project.git",
				SourceBranch: "main",
				RelativePath: "guide.md",
				DocsBase:     "docs",
			},
			expectedURL: "https://gitea.example.com/org/project/_edit/main/docs/guide.md",
		},
		{
			name: "fallback to main when no branch",
			doc: &Document{
				Forge:        "github",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "",
				RelativePath: "file.md",
				DocsBase:     "",
			},
			expectedURL: "https://github.com/org/repo/edit/main/file.md",
		},
		{
			name: "detect github from URL",
			doc: &Document{
				Forge:        "", // No explicit forge
				SourceURL:    "https://github.com/org/repo.git",
				SourceBranch: "main",
				RelativePath: "doc.md",
				DocsBase:     "",
			},
			expectedURL: "https://github.com/org/repo/edit/main/doc.md",
		},
		{
			name: "detect gitlab from URL",
			doc: &Document{
				Forge:        "", // No explicit forge
				SourceURL:    "https://gitlab.example.com/group/project",
				SourceBranch: "main",
				RelativePath: "doc.md",
				DocsBase:     "",
			},
			expectedURL: "https://gitlab.example.com/group/project/-/edit/main/doc.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualURL := generateEditURL(tt.doc)
			assert.Equal(t, tt.expectedURL, actualURL)
		})
	}
}

// TestGenerateEditURL_WithEditURLBase verifies EditURLBase override works correctly.
func TestGenerateEditURL_WithEditURLBase(t *testing.T) {
	tests := []struct {
		name        string
		doc         *Document
		expectedURL string
	}{
		{
			name: "EditURLBase overrides SourceURL",
			doc: &Document{
				Forge:        "gitlab",
				SourceURL:    "https://gitlab.com/old/repo.git",
				EditURLBase:  "https://gitlab.example.com/group/project",
				SourceBranch: "main",
				RelativePath: "guide.md",
				DocsBase:     "docs",
			},
			expectedURL: "https://gitlab.example.com/group/project/-/edit/main/docs/guide.md",
		},
		{
			name: "EditURLBase without SourceURL",
			doc: &Document{
				Forge:        "github",
				SourceURL:    "", // No source URL (local repo)
				EditURLBase:  "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "api.md",
				DocsBase:     "documentation",
			},
			expectedURL: "https://github.com/org/repo/edit/main/documentation/api.md",
		},
		{
			name: "EditURLBase with .git suffix stripped",
			doc: &Document{
				Forge:        "github",
				EditURLBase:  "https://github.com/org/repo.git",
				SourceBranch: "develop",
				RelativePath: "README.md",
				DocsBase:     "",
			},
			expectedURL: "https://github.com/org/repo/edit/develop/README.md",
		},
		{
			name: "EditURLBase with subdirectory path",
			doc: &Document{
				Forge:        "gitlab",
				EditURLBase:  "https://gitlab.com/group/project",
				SourceBranch: "main",
				RelativePath: "api/reference.md",
				DocsBase:     "docs",
			},
			expectedURL: "https://gitlab.com/group/project/-/edit/main/docs/api/reference.md",
		},
		{
			name: "EditURLBase with DocsBase as current directory (.)",
			doc: &Document{
				Forge:        "gitlab",
				EditURLBase:  "https://gitlab.example.com/group/project",
				SourceBranch: "main",
				RelativePath: "ci-cd-setup.md",
				DocsBase:     ".",
			},
			expectedURL: "https://gitlab.example.com/group/project/-/edit/main/ci-cd-setup.md",
		},
		{
			name: "Local build mode with DocsBase as dot",
			doc: &Document{
				Forge:        "github",
				EditURLBase:  "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "README.md",
				DocsBase:     ".",
			},
			expectedURL: "https://github.com/org/repo/edit/main/README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualURL := generateEditURL(tt.doc)
			assert.Equal(t, tt.expectedURL, actualURL)
		})
	}
}

// TestBuildBaseFrontMatter_Idempotent verifies that base frontmatter building is idempotent.
func TestBuildBaseFrontMatter_Idempotent(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "empty frontmatter",
			doc: &Document{
				FrontMatter: make(map[string]any),
				Name:        "test-document",
			},
		},
		{
			name: "existing title",
			doc: &Document{
				FrontMatter: map[string]any{
					"title": "Custom Title",
				},
				Name: "test-document",
			},
		},
		{
			name: "existing type",
			doc: &Document{
				FrontMatter: map[string]any{
					"type": "blog",
				},
				Name: "test-document",
			},
		},
		{
			name: "with source info for edit link",
			doc: &Document{
				FrontMatter:  make(map[string]any),
				Name:         "guide",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "docs/guide.md",
				DocsBase:     "docs",
				Forge:        "github",
			},
		},
		{
			name: "existing editURL should be preserved",
			doc: &Document{
				FrontMatter: map[string]any{
					"editURL": "https://custom.example.com/edit",
				},
				Name:         "guide",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "docs/guide.md",
				DocsBase:     "docs",
				Forge:        "github",
			},
		},
		{
			name: "all fields already present",
			doc: &Document{
				FrontMatter: map[string]any{
					"title":   "Custom Title",
					"type":    "custom",
					"editURL": "https://custom.example.com/edit",
				},
				Name:         "document",
				SourceURL:    "https://github.com/org/repo",
				SourceBranch: "main",
				RelativePath: "doc.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy for the second run
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := buildBaseFrontMatter(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture state after first application
			state1 := captureDocumentState(doc1)

			// Apply transform second time
			newDocs2, err2 := buildBaseFrontMatter(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture state after second application
			state2 := captureDocumentState(doc1)

			// States should be identical (idempotent)
			assert.Equal(t, state1, state2, "applying transform twice should produce same result")

			// Also verify independent application produces same result
			newDocs3, err3 := buildBaseFrontMatter(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			state3 := captureDocumentState(doc2)
			assert.Equal(t, state1, state3, "independent application should produce same result")
		})
	}
}

// TestParseFrontMatter_Idempotent verifies that parsing frontmatter is idempotent.
func TestParseFrontMatter_Idempotent(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "with valid frontmatter",
			doc: &Document{
				Content: "---\ntitle: Test Document\ntype: docs\n---\n\n# Content\n\nBody text here.",
			},
		},
		{
			name: "no frontmatter",
			doc: &Document{
				Content: "# Content\n\nBody text without frontmatter.",
			},
		},
		{
			name: "empty frontmatter",
			doc: &Document{
				Content: "---\n---\n\n# Content\n\nBody text.",
			},
		},
		{
			name: "frontmatter with windows line endings",
			doc: &Document{
				Content: "---\r\ntitle: Windows Doc\r\n---\r\n\r\n# Content\r\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy for the second run
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := parseFrontMatter(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture state after first application
			state1 := captureDocumentStateWithFrontMatterParsed(doc1)

			// Apply transform second time (parsing already-parsed document)
			newDocs2, err2 := parseFrontMatter(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture state after second application
			state2 := captureDocumentStateWithFrontMatterParsed(doc1)

			// States should be identical (idempotent)
			assert.Equal(t, state1, state2, "applying transform twice should produce same result")

			// Also verify independent application produces same result
			newDocs3, err3 := parseFrontMatter(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			state3 := captureDocumentStateWithFrontMatterParsed(doc2)
			assert.Equal(t, state1, state3, "independent application should produce same result")
		})
	}
}

// TestSerializeDocument_Idempotent verifies that serialization is idempotent.
// Note: This is a special case - serialization modifies Content and Raw,
// so we verify that serializing an already-serialized document produces the same output.
func TestSerializeDocument_Idempotent(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
	}{
		{
			name: "with frontmatter",
			doc: &Document{
				Content: "# Body content\n\nSome text.",
				FrontMatter: map[string]any{
					"title": "Test",
					"type":  "docs",
				},
			},
		},
		{
			name: "no frontmatter",
			doc: &Document{
				Content:     "# Body content\n\nSome text.",
				FrontMatter: make(map[string]any),
			},
		},
		{
			name: "complex frontmatter",
			doc: &Document{
				Content: "# Content here",
				FrontMatter: map[string]any{
					"title":      "Complex Document",
					"type":       "docs",
					"tags":       []string{"tag1", "tag2"},
					"repository": "test-repo",
					"editURL":    "https://example.com/edit",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy for the second run
			doc1 := cloneDocument(tt.doc)
			doc2 := cloneDocument(tt.doc)

			// Apply transform once
			newDocs1, err1 := serializeDocument(doc1)
			require.NoError(t, err1)
			assert.Nil(t, newDocs1, "should not generate new documents")

			// Capture serialized output
			firstContent := doc1.Content
			firstRaw := string(doc1.Raw)

			// Apply transform second time
			newDocs2, err2 := serializeDocument(doc1)
			require.NoError(t, err2)
			assert.Nil(t, newDocs2, "should not generate new documents on second run")

			// Capture serialized output again
			secondContent := doc1.Content
			secondRaw := string(doc1.Raw)

			// Outputs should be identical (idempotent)
			assert.Equal(t, firstContent, secondContent, "serializing twice should produce same Content")
			assert.Equal(t, firstRaw, secondRaw, "serializing twice should produce same Raw")

			// Also verify independent application produces same result
			newDocs3, err3 := serializeDocument(doc2)
			require.NoError(t, err3)
			assert.Nil(t, newDocs3)
			assert.Equal(t, firstContent, doc2.Content, "independent serialization should produce same result")
			assert.Equal(t, firstRaw, string(doc2.Raw), "independent serialization Raw should match")
		})
	}
}

// Helper functions

// cloneDocument creates a deep copy of a document for testing.
func cloneDocument(doc *Document) *Document {
	clone := &Document{
		Content:             doc.Content,
		FrontMatter:         make(map[string]any),
		OriginalFrontMatter: make(map[string]any),
		Path:                doc.Path,
		Repository:          doc.Repository,
		Forge:               doc.Forge,
		Section:             doc.Section,
		Name:                doc.Name,
		Extension:           doc.Extension,
		IsIndex:             doc.IsIndex,
		Generated:           doc.Generated,
		HadFrontMatter:      doc.HadFrontMatter,
		SourceURL:           doc.SourceURL,
		SourceCommit:        doc.SourceCommit,
		SourceBranch:        doc.SourceBranch,
		RelativePath:        doc.RelativePath,
		DocsBase:            doc.DocsBase,
		FilePath:            doc.FilePath,
	}

	// Deep copy maps
	maps.Copy(clone.FrontMatter, doc.FrontMatter)
	maps.Copy(clone.OriginalFrontMatter, doc.OriginalFrontMatter)

	return clone
}

// captureDocumentState captures relevant document state for comparison.
type documentState struct {
	FrontMatter map[string]any
	Content     string
}

func captureDocumentState(doc *Document) documentState {
	state := documentState{
		FrontMatter: make(map[string]any),
		Content:     doc.Content,
	}

	// Deep copy frontmatter
	maps.Copy(state.FrontMatter, doc.FrontMatter)

	return state
}

// documentStateWithParsed captures state including parsed frontmatter fields.
type documentStateWithParsed struct {
	FrontMatter         map[string]any
	OriginalFrontMatter map[string]any
	Content             string
	HadFrontMatter      bool
}

func captureDocumentStateWithFrontMatterParsed(doc *Document) documentStateWithParsed {
	state := documentStateWithParsed{
		FrontMatter:         make(map[string]any),
		OriginalFrontMatter: make(map[string]any),
		Content:             doc.Content,
		HadFrontMatter:      doc.HadFrontMatter,
	}

	// Deep copy frontmatter
	maps.Copy(state.FrontMatter, doc.FrontMatter)
	maps.Copy(state.OriginalFrontMatter, doc.OriginalFrontMatter)

	return state
}
