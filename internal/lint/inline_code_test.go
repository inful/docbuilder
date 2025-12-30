package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInsideInlineCode(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		pos      int
		expected bool
	}{
		{
			name:     "before any backticks",
			line:     "Some text `code` more text",
			pos:      5,
			expected: false,
		},
		{
			name:     "inside inline code",
			line:     "Some text `code` more text",
			pos:      13,
			expected: true,
		},
		{
			name:     "after inline code",
			line:     "Some text `code` more text",
			pos:      20,
			expected: false,
		},
		{
			name:     "multiple inline code blocks - first",
			line:     "Text `code1` and `code2` end",
			pos:      8,
			expected: true,
		},
		{
			name:     "multiple inline code blocks - between",
			line:     "Text `code1` and `code2` end",
			pos:      15,
			expected: false,
		},
		{
			name:     "multiple inline code blocks - second",
			line:     "Text `code1` and `code2` end",
			pos:      20,
			expected: true,
		},
		{
			name:     "link inside backticks",
			line:     "Use `[Link](./file.md)` syntax",
			pos:      10,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInsideInlineCode(tt.line, tt.pos)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectBrokenLinks_SkipsInlineCode(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	// Create a file with links inside inline code (should NOT be flagged as broken)
	indexPath := filepath.Join(docsDir, "index.md")
	content := `# Documentation

Regular broken link: [Missing](./missing.md)

Inline code with link syntax: ` + "`[Link](./guide.md)`" + `

Code block:
` + "```" + `
[Another Link](./another.md)
` + "```" + `

Image in inline code: ` + "`![Image](./image.png)`" + `

Reference link in inline code: ` + "`[ref]: ./reference.md`" + `

Real image: ![Real](./missing.png)
`
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o644))

	// Run broken link detection
	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	// Should only find 2 broken links:
	// 1. [Missing](./missing.md) - real link
	// 2. ![Real](./missing.png) - real image
	// Should NOT find links/images inside backticks or code blocks
	assert.Len(t, broken, 2, "should only detect real broken links, not those in inline code")

	var targets = make([]string, 0, len(broken))
	for _, bl := range broken {
		targets = append(targets, bl.Target)
	}
	assert.Contains(t, targets, "./missing.md")
	assert.Contains(t, targets, "./missing.png")
	assert.NotContains(t, targets, "./guide.md", "should skip link in inline code")
	assert.NotContains(t, targets, "./another.md", "should skip link in code block")
	assert.NotContains(t, targets, "./image.png", "should skip image in inline code")
	assert.NotContains(t, targets, "./reference.md", "should skip reference in inline code")
}

func TestDetectBrokenLinks_ComplexInlineCode(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	// Test edge cases with inline code
	indexPath := filepath.Join(docsDir, "test.md")
	content := `# Test

Multiple backticks on same line: ` + "`code1` and `code2` and [Real Link](./missing.md)`" + `

Nested situation: Use ` + "`[Example](./example.md)`" + ` like this [Broken](./broken.md)

Backtick at end: ` + "`something` [Link](./real-missing.md)" + `
`
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o644))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	// Should find 2 broken links: ./missing.md and ./broken.md and ./real-missing.md
	// Should NOT find ./example.md (inside backticks)
	var targets = make([]string, 0, len(broken))
	for _, bl := range broken {
		targets = append(targets, bl.Target)
	}

	assert.Contains(t, targets, "./missing.md")
	assert.Contains(t, targets, "./broken.md")
	assert.Contains(t, targets, "./real-missing.md")
	assert.NotContains(t, targets, "./example.md", "should skip link inside backticks")
}
