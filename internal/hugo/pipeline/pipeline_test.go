package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestDocument_NewFromDocFile(t *testing.T) {
	// Test that we can create a Document from a DocFile
	// This is a placeholder test to ensure the package compiles
	doc := &Document{
		Content:     "# Test\n\nThis is test content",
		FrontMatter: make(map[string]any),
		IsIndex:     false,
		Repository:  "test-repo",
		Path:        "test-repo/test.md",
	}

	if doc.Content == "" {
		t.Error("Expected content to be set")
	}

	if doc.Repository != "test-repo" {
		t.Errorf("Expected repository 'test-repo', got '%s'", doc.Repository)
	}
}

func TestProcessor_New(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:       "Test Site",
			Description: "Test Description",
		},
	}

	processor := NewProcessor(cfg)
	if processor == nil {
		t.Fatal("Expected processor to be created")
	}

	if processor.config != cfg {
		t.Error("Expected config to be set")
	}

	if len(processor.generators) == 0 {
		t.Error("Expected default generators to be set")
	}

	if len(processor.transforms) == 0 {
		t.Error("Expected default transforms to be set")
	}
}

func TestParseFrontMatter(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectFM      bool
		expectTitle   string
		expectContent string
	}{
		{
			name: "valid front matter",
			content: `---
title: Test Page
description: Test description
---
# Content

This is the body.`,
			expectFM:      true,
			expectTitle:   "Test Page",
			expectContent: "# Content\n\nThis is the body.",
		},
		{
			name:          "no front matter",
			content:       "# Just Content\n\nNo front matter here.",
			expectFM:      false,
			expectTitle:   "",
			expectContent: "# Just Content\n\nNo front matter here.",
		},
		{
			name: "empty front matter",
			content: `---
---
# Content`,
			expectFM:      false,
			expectTitle:   "",
			expectContent: "# Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{
				Content:     tt.content,
				FrontMatter: make(map[string]any),
			}

			_, err := parseFrontMatter(doc)
			if err != nil {
				t.Fatalf("parseFrontMatter failed: %v", err)
			}

			if doc.HadFrontMatter != tt.expectFM {
				t.Errorf("Expected HadFrontMatter=%v, got %v", tt.expectFM, doc.HadFrontMatter)
			}

			if tt.expectFM && tt.expectTitle != "" {
				if title, ok := doc.FrontMatter["title"].(string); !ok || title != tt.expectTitle {
					t.Errorf("Expected title=%q, got %q", tt.expectTitle, title)
				}
			}

			if doc.Content != tt.expectContent {
				t.Errorf("Expected content=%q, got %q", tt.expectContent, doc.Content)
			}
		})
	}
}

func TestExtractIndexTitle(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		isIndex       bool
		expectTitle   string
		shouldExtract bool
	}{
		{
			name:          "index with H1 at start",
			content:       "# My Title\n\nContent here",
			isIndex:       true,
			expectTitle:   "My Title",
			shouldExtract: true,
		},
		{
			name:          "index with text before H1",
			content:       "Some text\n# My Title\n\nContent",
			isIndex:       true,
			shouldExtract: false,
		},
		{
			name:          "non-index file",
			content:       "# My Title\n\nContent",
			isIndex:       false,
			shouldExtract: false,
		},
		{
			name:          "no H1",
			content:       "Just content, no heading",
			isIndex:       true,
			shouldExtract: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{
				Content:     tt.content,
				IsIndex:     tt.isIndex,
				FrontMatter: make(map[string]any),
			}

			_, err := extractIndexTitle(doc)
			if err != nil {
				t.Fatalf("extractIndexTitle failed: %v", err)
			}

			if tt.shouldExtract {
				title, ok := doc.FrontMatter["title"].(string)
				if !ok {
					t.Error("Expected title to be extracted")
				} else if title != tt.expectTitle {
					t.Errorf("Expected title=%q, got %q", tt.expectTitle, title)
				}
			}
		})
	}
}

func TestSerializeDocument(t *testing.T) {
	doc := &Document{
		Content: "# Test Content\n\nThis is the body.",
		FrontMatter: map[string]any{
			"title":       "Test Page",
			"description": "Test description",
		},
	}

	_, err := serializeDocument(doc)
	if err != nil {
		t.Fatalf("serializeDocument failed: %v", err)
	}

	if len(doc.Raw) == 0 {
		t.Error("Expected Raw to be populated")
	}

	// Check that it contains front matter delimiter
	content := string(doc.Raw)
	if !containsString(content, "---") {
		t.Error("Expected serialized content to contain front matter delimiters")
	}

	if !containsString(content, "title: Test Page") {
		t.Error("Expected serialized content to contain title")
	}

	if !containsString(content, "# Test Content") {
		t.Error("Expected serialized content to contain body")
	}
}

func TestGenerateMainIndex(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:       "My Documentation",
			Description: "My documentation site",
		},
	}

	ctx := &GenerationContext{
		Discovered: []*Document{},
		Config:     cfg,
	}

	docs, err := generateMainIndex(ctx)
	if err != nil {
		t.Fatalf("generateMainIndex failed: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("Expected 1 generated document, got %d", len(docs))
	}

	doc := docs[0]
	if !doc.Generated {
		t.Error("Expected document to be marked as Generated")
	}

	if !doc.IsIndex {
		t.Error("Expected document to be marked as IsIndex")
	}

	if doc.Path != "content/_index.md" {
		t.Errorf("Expected path='content/_index.md', got %q", doc.Path)
	}

	if title, ok := doc.FrontMatter["title"].(string); !ok || title != "My Documentation" {
		t.Errorf("Expected title='My Documentation', got %q", title)
	}
}

// Helper function.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
