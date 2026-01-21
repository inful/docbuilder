package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"

func TestFixer_FixUID_GeneratesUIDAndAlias(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
title: "Test Document"
---

# Test Document

Content here.
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false) // dryRun=false, force=false
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify file was modified
	// #nosec G304 -- filePath is from test temp directory
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "uid:")
	assert.Contains(t, contentStr, "aliases:")
	assert.Contains(t, contentStr, "/_uid/")
}

func TestFixer_FixUID_AddsAliasToExistingUID(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	existingUID := testUUID
	content := `---
uid: ` + existingUID + `
title: "Test Document"
---

# Test Document
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify alias was added
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "uid: "+existingUID)
	assert.Contains(t, contentStr, "aliases:")
	assert.Contains(t, contentStr, "/_uid/"+existingUID+"/")
}

func TestFixer_FixUID_PreservesExistingAliases(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	existingUID := testUUID
	content := `---
uid: ` + existingUID + `
title: "Test Document"
aliases:
  - /old/path/
  - /another/old/path/
---

# Test Document
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify existing aliases were preserved and uid-based alias was added
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "/old/path/")
	assert.Contains(t, contentStr, "/another/old/path/")
	assert.Contains(t, contentStr, "/_uid/"+existingUID+"/")
}

func TestFixer_FixUID_SkipsIndexFiles(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "_index.md")

	content := `---
title: "Section Index"
---

# Section Index
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	_, err = fixer.Fix(filePath)
	require.NoError(t, err)
	// _index.md files skip UID validation but may still get fingerprint fixes
	// Just verify no UID was added
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	contentStr := string(modifiedContent)
	assert.NotContains(t, contentStr, "uid:")
	assert.NotContains(t, contentStr, "aliases:")
}

func TestFixer_FixUID_DryRunDoesNotModify(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	originalContent := `---
title: "Test Document"
---

# Test Document
`
	err := os.WriteFile(filePath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, true, false) // dryRun=true
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges()) // Would have changes

	// Verify file was NOT actually modified
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(modifiedContent))
}

func TestFixer_FixUID_HandlesStringAlias(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	existingUID := testUUID
	content := `---
uid: ` + existingUID + `
title: "Test Document"
aliases: /old/path/
---

# Test Document
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify both aliases exist
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "/old/path/")
	assert.Contains(t, contentStr, "/_uid/"+existingUID+"/")
}

func TestFixer_FixUID_SkipsIfAliasAlreadyExists(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	existingUID := testUUID
	content := `---
uid: ` + existingUID + `
title: "Test Document"
aliases:
  - /_uid/` + existingUID + `/
---

# Test Document
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	_, err = fixer.Fix(filePath)
	require.NoError(t, err)
	// File already has uid and correct alias, so no UID/alias changes needed
	// May have fingerprint updates though
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	contentStr := string(modifiedContent)
	// Verify UID and alias are unchanged
	assert.Contains(t, contentStr, "uid: "+existingUID)
	assert.Contains(t, contentStr, "/_uid/"+existingUID+"/")
}

func TestFixer_FixUID_GeneratesValidUUID(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
title: "Test Document"
---

# Test Document
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify UID is a valid UUID format (8-4-4-4-12)
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	lines := strings.Split(contentStr, "\n")
	var uidLine string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "uid:") {
			uidLine = line
			break
		}
	}

	require.NotEmpty(t, uidLine, "UID line not found")
	uidValue := strings.TrimSpace(strings.TrimPrefix(uidLine, "uid:"))

	// Check UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	parts := strings.Split(uidValue, "-")
	require.Len(t, parts, 5, "UID should have 5 parts separated by hyphens")
	assert.Len(t, parts[0], 8)
	assert.Len(t, parts[1], 4)
	assert.Len(t, parts[2], 4)
	assert.Len(t, parts[3], 4)
	assert.Len(t, parts[4], 12)
}

func TestFixer_FixUID_PreservesContent(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
title: "Test Document"
date: 2024-01-01
tags:
  - documentation
  - testing
---

# Test Document

This is the main content.

## Subsection

With multiple paragraphs.

- Bullet points
- More bullets

Some **bold** and *italic* text.
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify content after frontmatter is preserved
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "# Test Document")
	assert.Contains(t, contentStr, "This is the main content.")
	assert.Contains(t, contentStr, "## Subsection")
	assert.Contains(t, contentStr, "- Bullet points")
	assert.Contains(t, contentStr, "Some **bold** and *italic* text.")

	// Verify other frontmatter fields are preserved
	assert.Contains(t, contentStr, "title: Test Document")
	assert.Contains(t, contentStr, "date: 2024-01-01")
	assert.Contains(t, contentStr, "tags:")
	assert.Contains(t, contentStr, "- documentation")
	assert.Contains(t, contentStr, "- testing")
}

func TestFixer_FixUID_HandlesComplexFrontmatter(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
title: "Complex Document"
author:
  name: "John Doe"
  email: "john@example.com"
metadata:
  category: "testing"
  priority: high
  tags:
    - go
    - testing
    - automation
related:
  - link: "/doc1"
    title: "Doc 1"
  - link: "/doc2"
    title: "Doc 2"
---

# Content
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify all complex frontmatter is preserved
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "author:")
	assert.Contains(t, contentStr, "name: John Doe")
	assert.Contains(t, contentStr, "email: john@example.com")
	assert.Contains(t, contentStr, "metadata:")
	assert.Contains(t, contentStr, "category: testing")
	assert.Contains(t, contentStr, "priority: high")
	assert.Contains(t, contentStr, "related:")
	assert.Contains(t, contentStr, "link: /doc1")

	// Verify uid and aliases were added
	assert.Contains(t, contentStr, "uid:")
	assert.Contains(t, contentStr, "aliases:")
	assert.Contains(t, contentStr, "/_uid/")
}

func TestFixer_FixUID_NoFrontmatter(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `# Test Document

No frontmatter at all.
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	result, err := fixer.Fix(filePath)
	require.NoError(t, err)
	assert.True(t, result.HasChanges())

	// Verify frontmatter was created
	// #nosec G304 -- filePath is from test temp directory
	modifiedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "---")
	assert.Contains(t, contentStr, "uid:")
	assert.Contains(t, contentStr, "aliases:")
	assert.Contains(t, contentStr, "# Test Document")
}

func TestFixer_FixUID_BatchMode(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple files
	files := []string{"doc1.md", "doc2.md", "doc3.md"}
	for _, filename := range files {
		filePath := filepath.Join(tempDir, filename)
		content := `---
title: "` + filename + `"
---

# ` + filename + `
`
		err := os.WriteFile(filePath, []byte(content), 0o600)
		require.NoError(t, err)
	}

	// Fix all files
	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false)
	for _, filename := range files {
		filePath := filepath.Join(tempDir, filename)
		result, err := fixer.Fix(filePath)
		require.NoError(t, err)
		assert.True(t, result.HasChanges(), "File %s should have changes", filename)
	}

	// Verify all files have unique UIDs
	uids := make(map[string]bool)
	for _, filename := range files {
		filePath := filepath.Join(tempDir, filename)
		// #nosec G304 -- filePath is from test temp directory
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		contentStr := string(content)
		//nolint:modernize // strings.Split is clearer for test code
		lines := strings.Split(contentStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "uid:") {
				uidValue := strings.TrimSpace(strings.TrimPrefix(line, "uid:"))
				assert.False(t, uids[uidValue], "UID %s is duplicated in %s", uidValue, filename)
				uids[uidValue] = true
			}
		}
	}

	assert.Len(t, uids, len(files), "Each file should have a unique UID")
}
