package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectBrokenLinks_SkipsTildeFencedCodeBlocks(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Links inside fenced code blocks should not contribute to broken-link errors.
	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n" +
		"~~~go\n" +
		"[NotARealBrokenLink](./missing.md)\n" +
		"~~~\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	assert.Empty(t, broken)
}

func TestDetectBrokenLinks_AllowsNestedParenthesesInInlineLinkTargets(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Inline links with parentheses in the destination should parse correctly.
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "file(name).md"), []byte("# ok\n"), 0o600))

	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n[HasParens](./file(name).md)\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	assert.Empty(t, broken)
}

func TestDetectBrokenLinks_IgnoresEscapedLinkText(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Escaped link text should not be treated as a link.
	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n\\[NotALink](./missing.md)\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	assert.Empty(t, broken)
}

func TestDetectBrokenLinks_ReferenceDefinitionIsChecked(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n[ref]: ./missing.md\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	require.Len(t, broken, 1)
	assert.Equal(t, LinkTypeReference, broken[0].LinkType)
	assert.Equal(t, "./missing.md", broken[0].Target)
}
