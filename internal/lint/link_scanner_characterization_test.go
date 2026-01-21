package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectBrokenLinks_KnownLimitation_TildeFencedCodeBlocksNotSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Known limitation: detectBrokenLinksInFile only toggles code-block mode on ``` fences,
	// so links inside ~~~ fenced blocks are still scanned.
	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n" +
		"~~~go\n" +
		"[NotARealBrokenLink](./missing.md)\n" +
		"~~~\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	require.Len(t, broken, 1)
	assert.Equal(t, "./missing.md", broken[0].Target)
}

func TestDetectBrokenLinks_KnownLimitation_NestedParenthesesInLinkTarget(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Create a file that should make the link valid.
	// Known limitation: inline link parsing uses the first ')' to terminate the destination,
	// so './file(name).md' is parsed as './file(name' and flagged as broken.
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "file(name).md"), []byte("# ok\n"), 0o600))

	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n[HasParens](./file(name).md)\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	require.Len(t, broken, 1)
	assert.Equal(t, "./file(name", broken[0].Target)
}

func TestDetectBrokenLinks_KnownLimitation_EscapedLinkTextDetected(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Known limitation: the scanner does not account for Markdown escaping, so an escaped
	// opening bracket still participates in link detection.
	indexPath := filepath.Join(docsDir, "index.md")
	content := "# Test\n\n\\[NotALink](./missing.md)\n"
	require.NoError(t, os.WriteFile(indexPath, []byte(content), 0o600))

	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	require.Len(t, broken, 1)
	assert.Equal(t, "./missing.md", broken[0].Target)
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
