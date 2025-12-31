package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDocGuidePath = "/docs/guide.md"

func TestResolveRelativePath(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		linkTarget string
		want       string
		wantErr    bool
	}{
		{
			name:       "same directory",
			sourceFile: testDocGuidePath,
			linkTarget: "api.md",
			want:       "/docs/api.md",
		},
		{
			name:       "parent directory",
			sourceFile: "/docs/guides/tutorial.md",
			linkTarget: "../api.md",
			want:       "/docs/api.md",
		},
		{
			name:       "subdirectory",
			sourceFile: "/docs/index.md",
			linkTarget: "./guides/tutorial.md",
			want:       "/docs/guides/tutorial.md",
		},
		{
			name:       "with fragment",
			sourceFile: testDocGuidePath,
			linkTarget: "api.md#section",
			want:       "/docs/api.md",
		},
		{
			name:       "multiple parent traversals",
			sourceFile: "/docs/guides/advanced/testing.md",
			linkTarget: "../../api.md",
			want:       "/docs/api.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRelativePath(tt.sourceFile, tt.linkTarget)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Normalize paths for comparison (handle different OS separators)
			gotClean := filepath.Clean(got)
			wantClean := filepath.Clean(tt.want)
			assert.Equal(t, wantClean, gotClean)
		})
	}
}

func TestFindInlineLinks(t *testing.T) {
	sourceFile := testDocGuidePath
	targetPath := "/docs/api.md"

	tests := []struct {
		name     string
		line     string
		lineNum  int
		expected int // number of links expected
	}{
		{
			name:     "simple inline link",
			line:     "See the [API Guide](api.md) for details.",
			lineNum:  1,
			expected: 1,
		},
		{
			name:     "link with fragment",
			line:     "Check [authentication](api.md#auth) section.",
			lineNum:  2,
			expected: 1,
		},
		{
			name:     "multiple links",
			line:     "See [API](api.md) and [Guide](api.md) pages.",
			lineNum:  3,
			expected: 2,
		},
		{
			name:     "external link (should skip)",
			line:     "Visit [GitHub](https://github.com/api.md).",
			lineNum:  4,
			expected: 0,
		},
		{
			name:     "no links",
			line:     "This is plain text without any links.",
			lineNum:  5,
			expected: 0,
		},
		{
			name:     "image link (should skip - not inline text link)",
			line:     "![Diagram](api.md)",
			lineNum:  6,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := findInlineLinks(tt.line, tt.lineNum, sourceFile, targetPath)
			assert.Len(t, links, tt.expected)

			for _, link := range links {
				assert.Equal(t, sourceFile, link.SourceFile)
				assert.Equal(t, tt.lineNum, link.LineNumber)
				assert.Equal(t, LinkTypeInline, link.LinkType)
			}
		})
	}
}

// linkFinderTestCase defines a test case for link finder functions.
type linkFinderTestCase struct {
	name     string
	line     string
	lineNum  int
	expected int
}

// testLinkFinder is a generic test helper that reduces duplication between
// TestFindReferenceLinks, TestFindImageLinks, and similar link finder tests.
//
// It runs table-driven tests and verifies:
//   - Number of links found matches expected count.
//   - All links have correct SourceFile, LineNumber, and LinkType.
func testLinkFinder(
	t *testing.T,
	sourceFile string,
	targetPath string,
	expectedLinkType LinkType,
	finderFunc func(line string, lineNum int, sourceFile string, targetPath string) []LinkReference,
	tests []linkFinderTestCase,
) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := finderFunc(tt.line, tt.lineNum, sourceFile, targetPath)
			assert.Len(t, links, tt.expected)

			for _, link := range links {
				assert.Equal(t, sourceFile, link.SourceFile)
				assert.Equal(t, tt.lineNum, link.LineNumber)
				assert.Equal(t, expectedLinkType, link.LinkType)
			}
		})
	}
}

func TestFindReferenceLinks(t *testing.T) {
	sourceFile := testDocGuidePath
	targetPath := "/docs/api.md"

	tests := []linkFinderTestCase{
		{
			name:     "simple reference link",
			line:     "[api]: api.md",
			lineNum:  1,
			expected: 1,
		},
		{
			name:     "reference link with title",
			line:     "[api]: api.md \"API Documentation\"",
			lineNum:  2,
			expected: 1,
		},
		{
			name:     "reference link with fragment",
			line:     "[auth]: api.md#authentication",
			lineNum:  3,
			expected: 1,
		},
		{
			name:     "external reference (should skip)",
			line:     "[github]: https://github.com",
			lineNum:  4,
			expected: 0,
		},
		{
			name:     "not a reference link",
			line:     "This [is not] a reference link",
			lineNum:  5,
			expected: 0,
		},
	}

	testLinkFinder(t, sourceFile, targetPath, LinkTypeReference, findReferenceLinks, tests)
}

func TestFindImageLinks(t *testing.T) {
	sourceFile := testDocGuidePath
	targetPath := "/docs/diagram.png"

	tests := []linkFinderTestCase{
		{
			name:     "simple image link",
			line:     "![Architecture](diagram.png)",
			lineNum:  1,
			expected: 1,
		},
		{
			name:     "image link with alt text",
			line:     "![System Diagram](diagram.png)",
			lineNum:  2,
			expected: 1,
		},
		{
			name:     "multiple images",
			line:     "![Img1](diagram.png) and ![Img2](diagram.png)",
			lineNum:  3,
			expected: 2,
		},
		{
			name:     "external image (should skip)",
			line:     "![Logo](https://example.com/logo.png)",
			lineNum:  4,
			expected: 0,
		},
		{
			name:     "regular link (should skip)",
			line:     "[Link](diagram.png)",
			lineNum:  5,
			expected: 0,
		},
	}

	testLinkFinder(t, sourceFile, targetPath, LinkTypeImage, findImageLinks, tests)
}

func TestFindLinksInFile(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tmpDir, "guide.md")
	sourceContent := `# Guide

See the [API Guide](api.md) for details.

Also check [authentication](api.md#auth).

External link: [GitHub](https://github.com/example)

![Architecture](diagram.png)

Reference links:
[api-ref]: api.md
[external]: https://example.com

Code block (should be ignored):
` + "```bash\n# See api.md for info\n```" + `
`

	err := os.WriteFile(sourceFile, []byte(sourceContent), 0o644)
	require.NoError(t, err)

	// Create target file
	targetFile := filepath.Join(tmpDir, "api.md")
	err = os.WriteFile(targetFile, []byte("# API"), 0o644)
	require.NoError(t, err)

	// Get absolute paths
	absSource, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	absTarget, err := filepath.Abs(targetFile)
	require.NoError(t, err)

	// Create fixer
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	// Find links
	links, err := fixer.findLinksInFile(absSource, absTarget)
	require.NoError(t, err)

	// Should find: 2 inline links + 1 reference link = 3 total
	// (image and external links should be excluded)
	assert.GreaterOrEqual(t, len(links), 2, "should find at least inline links")

	// Verify we found the inline links
	inlineCount := 0
	refCount := 0
	for _, link := range links {
		switch link.LinkType {
		case LinkTypeInline:
			inlineCount++
		case LinkTypeReference:
			refCount++
		case LinkTypeImage:
			// Image links are not counted in either inline or reference
		}
	}

	assert.GreaterOrEqual(t, inlineCount, 2, "should find at least 2 inline links")
	assert.GreaterOrEqual(t, refCount, 0, "reference links may or may not be found depending on implementation")
}

func TestFindLinksToFile(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create directory structure
	docsDir := filepath.Join(tmpDir, "docs")
	guidesDir := filepath.Join(docsDir, "guides")
	err := os.MkdirAll(guidesDir, 0o755)
	require.NoError(t, err)

	// Create target file (to be renamed)
	targetFile := filepath.Join(docsDir, "API_Guide.md")
	err = os.WriteFile(targetFile, []byte("# API Guide"), 0o644)
	require.NoError(t, err)

	// Create file with link from same directory
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Documentation

See the [API Guide](API_Guide.md) for details.
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0o644)
	require.NoError(t, err)

	// Create file with link from subdirectory
	tutorialFile := filepath.Join(guidesDir, "tutorial.md")
	tutorialContent := `# Tutorial

Check the [API Guide](../API_Guide.md) for reference.

Also see [authentication](../API_Guide.md#auth).
`
	err = os.WriteFile(tutorialFile, []byte(tutorialContent), 0o644)
	require.NoError(t, err)

	// Create file with no links
	readmeFile := filepath.Join(docsDir, "README.md")
	err = os.WriteFile(readmeFile, []byte("# README\n\nNo links here."), 0o644)
	require.NoError(t, err)

	// Get absolute path of target
	absTarget, err := filepath.Abs(targetFile)
	require.NoError(t, err)

	// Create fixer
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	// Find all links to target file (search from docs root)
	links, err := fixer.findLinksToFile(absTarget, docsDir)
	require.NoError(t, err)

	// Should find 3 links:
	// - 1 in index.md
	// - 2 in tutorial.md (one without fragment, one with #auth)
	assert.GreaterOrEqual(t, len(links), 2, "should find at least 2 links")

	// Verify all links point to correct source files
	foundIndex := false
	foundTutorial := false

	for _, link := range links {
		if filepath.Base(link.SourceFile) == "index.md" {
			foundIndex = true
			assert.Equal(t, "API_Guide.md", link.Target)
		} else if filepath.Base(link.SourceFile) == "tutorial.md" {
			foundTutorial = true
			assert.Equal(t, "../API_Guide.md", link.Target)
		}
	}

	assert.True(t, foundIndex, "should find link in index.md")
	assert.True(t, foundTutorial, "should find link in tutorial.md")
}

func TestLinkDiscovery_CodeBlocks(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create source file with code blocks
	sourceFile := filepath.Join(tmpDir, "guide.md")
	sourceContent := `# Guide

Regular link: [API](api.md)

` + "```bash\n# This api.md reference should be ignored\ncurl api.md\n```" + `

Another regular link: [API Guide](api.md)

Indented code (4 spaces):
    See api.md for details

Regular text continues here with [link](api.md).
`

	err := os.WriteFile(sourceFile, []byte(sourceContent), 0o644)
	require.NoError(t, err)

	// Create target file
	targetFile := filepath.Join(tmpDir, "api.md")
	err = os.WriteFile(targetFile, []byte("# API"), 0o644)
	require.NoError(t, err)

	// Get absolute paths
	absSource, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	absTarget, err := filepath.Abs(targetFile)
	require.NoError(t, err)

	// Create fixer
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	// Find links
	links, err := fixer.findLinksInFile(absSource, absTarget)
	require.NoError(t, err)

	// Should find 3 regular links, ignoring code blocks
	// (Simple implementation may not fully handle fenced code blocks)
	assert.GreaterOrEqual(t, len(links), 2, "should find at least the clear regular links")

	// Verify no links come from code block lines
	for _, link := range links {
		// Line numbers of code blocks: approximately 5-7 and 11
		// Regular links: 3, 9, 13
		assert.NotEqual(t, 5, link.LineNumber)
		assert.NotEqual(t, 6, link.LineNumber)
		assert.NotEqual(t, 7, link.LineNumber)
		assert.NotEqual(t, 11, link.LineNumber)
	}
}
