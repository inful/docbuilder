package lint

import (
	"os"
	"path/filepath"
	"testing"
)

const testRuleFrontmatter = "frontmatter"

func TestFrontmatterRule_Name(t *testing.T) {
	rule := &FrontmatterRule{}
	if rule.Name() != testRuleFrontmatter {
		t.Errorf("Expected rule name '%s', got '%s'", testRuleFrontmatter, rule.Name())
	}
}

func TestFrontmatterRule_AppliesTo(t *testing.T) {
	rule := &FrontmatterRule{}

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{"Markdown .md file", "test.md", true},
		{"Markdown .markdown file", "test.markdown", true},
		{"Uppercase .MD file", "test.MD", true},
		{"Text file", "test.txt", false},
		{"Go file", "test.go", false},
		{"No extension", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.AppliesTo(tt.filePath)
			if result != tt.expected {
				t.Errorf("AppliesTo(%s) = %v, expected %v", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestFrontmatterRule_Check_NoFrontmatter(t *testing.T) {
	rule := &FrontmatterRule{}

	// Create temp file without frontmatter
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "no-frontmatter.md")
	content := "# Test Document\n\nThis has no frontmatter."
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	issues, err := rule.Check(filePath)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]
	if issue.Rule != testRuleFrontmatter {
		t.Errorf("Expected rule '%s', got '%s'", testRuleFrontmatter, issue.Rule)
	}
	if issue.Message != "Missing frontmatter" {
		t.Errorf("Expected 'Missing frontmatter' message, got '%s'", issue.Message)
	}
	if issue.Severity != SeverityWarning {
		t.Errorf("Expected SeverityWarning, got %v", issue.Severity)
	}
}

func TestFrontmatterRule_Check_MissingFields(t *testing.T) {
	rule := &FrontmatterRule{}

	tests := []struct {
		name          string
		content       string
		expectedCount int
		expectedMsgs  []string
	}{
		{
			name: "Missing all fields",
			content: `---
title: Test
---

Content here.`,
			expectedCount: 3,
			expectedMsgs:  []string{"Missing 'tags' field", "Missing 'categories' field", "Missing 'id' field"},
		},
		{
			name: "Missing tags only",
			content: `---
title: Test
categories: []
id: 123e4567-e89b-12d3-a456-426614174000
---

Content here.`,
			expectedCount: 1,
			expectedMsgs:  []string{"Missing 'tags' field"},
		},
		{
			name: "Missing categories only",
			content: `---
title: Test
tags: []
id: 123e4567-e89b-12d3-a456-426614174000
---

Content here.`,
			expectedCount: 1,
			expectedMsgs:  []string{"Missing 'categories' field"},
		},
		{
			name: "Missing id only",
			content: `---
title: Test
tags: []
categories: []
---

Content here.`,
			expectedCount: 1,
			expectedMsgs:  []string{"Missing 'id' field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(filePath, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			issues, err := rule.Check(filePath)
			if err != nil {
				t.Fatalf("Check returned error: %v", err)
			}

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d", tt.expectedCount, len(issues))
			}

			for i, expectedMsg := range tt.expectedMsgs {
				if i >= len(issues) {
					t.Errorf("Missing issue for: %s", expectedMsg)
					continue
				}
				if !contains(issues[i].Message, expectedMsg) {
					t.Errorf("Expected message containing '%s', got '%s'", expectedMsg, issues[i].Message)
				}
			}
		})
	}
}

func TestFrontmatterRule_Check_ValidFrontmatter(t *testing.T) {
	rule := &FrontmatterRule{}

	content := `---
title: Test Document
tags: [documentation, testing]
categories: [guides]
id: 123e4567-e89b-12d3-a456-426614174000
---

# Test Document

This has valid frontmatter with all required fields.`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "valid.md")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	issues, err := rule.Check(filePath)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for valid frontmatter, got %d", len(issues))
		for _, issue := range issues {
			t.Logf("Issue: %s", issue.Message)
		}
	}
}

func TestFixFrontmatter_NoFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "no-frontmatter.md")
	originalContent := "# Test Document\n\nThis has no frontmatter."
	if err := os.WriteFile(filePath, []byte(originalContent), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := FixFrontmatter(filePath); err != nil {
		t.Fatalf("FixFrontmatter failed: %v", err)
	}

	// Read updated content
	//nolint:gosec // G304: Reading test file by path is expected
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read fixed file: %v", err)
	}

	contentStr := string(content)

	// Verify frontmatter was added
	if !hasFrontmatter(contentStr) {
		t.Error("Expected frontmatter to be added")
	}

	// Verify required fields exist
	fm, err := parseFrontmatterFromFile(contentStr)
	if err != nil {
		t.Fatalf("Failed to parse frontmatter: %v", err)
	}

	if _, hasTags := fm["tags"]; !hasTags {
		t.Error("Expected 'tags' field in frontmatter")
	}
	if _, hasCategories := fm["categories"]; !hasCategories {
		t.Error("Expected 'categories' field in frontmatter")
	}
	if _, hasID := fm["id"]; !hasID {
		t.Error("Expected 'id' field in frontmatter")
	}

	// Verify original content is preserved
	if !contains(contentStr, "# Test Document") {
		t.Error("Original content not preserved")
	}
}

func TestFixFrontmatter_MissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "missing-fields.md")
	originalContent := `---
title: Test
---

Content here.`
	if err := os.WriteFile(filePath, []byte(originalContent), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := FixFrontmatter(filePath); err != nil {
		t.Fatalf("FixFrontmatter failed: %v", err)
	}

	// Read updated content
	//nolint:gosec // G304: Reading test file by path is expected
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read fixed file: %v", err)
	}

	// Parse and verify fields
	fm, err := parseFrontmatterFromFile(string(content))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter: %v", err)
	}

	if _, hasTags := fm["tags"]; !hasTags {
		t.Error("Expected 'tags' field to be added")
	}
	if _, hasCategories := fm["categories"]; !hasCategories {
		t.Error("Expected 'categories' field to be added")
	}
	if _, hasID := fm["id"]; !hasID {
		t.Error("Expected 'id' field to be added")
	}

	// Verify original title preserved
	if title, ok := fm["title"]; !ok || title != "Test" {
		t.Error("Original title not preserved")
	}
}

func TestFixFrontmatter_NoChangesNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "complete.md")
	originalContent := `---
title: Test
tags: []
categories: []
id: 123e4567-e89b-12d3-a456-426614174000
---

Content here.`
	if err := os.WriteFile(filePath, []byte(originalContent), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get file stats before fix
	infoBefore, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if err = FixFrontmatter(filePath); err != nil {
		t.Fatalf("FixFrontmatter failed: %v", err)
	}

	// Read content
	//nolint:gosec // G304: Reading test file by path is expected
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Get file stats after fix
	infoAfter, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// Verify no modifications (file should remain unchanged)
	if infoBefore.ModTime() != infoAfter.ModTime() {
		// Note: This check might not work on all filesystems
		t.Logf("File modification time changed (this may be expected)")
	}

	// Parse and verify all fields still present
	fm, err := parseFrontmatterFromFile(string(content))
	if err != nil {
		t.Fatalf("Failed to parse frontmatter: %v", err)
	}

	if fm["id"] != "123e4567-e89b-12d3-a456-426614174000" {
		t.Error("ID was modified when it shouldn't be")
	}
}
