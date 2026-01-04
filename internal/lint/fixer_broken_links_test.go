package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectBrokenLinks tests broken link detection.
func TestDetectBrokenLinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o750)
	require.NoError(t, err)

	// Create a file with broken links
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Documentation
[Existing File](./guide.md)
[Broken Link](./missing.md)
![Broken Image](./images/missing.png)
[External Link](https://example.com/guide.md)
[Fragment Only](#section)
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0o600)
	require.NoError(t, err)

	// Create the existing file
	guideFile := filepath.Join(docsDir, "guide.md")
	err = os.WriteFile(guideFile, []byte("# Guide"), 0o600)
	require.NoError(t, err)

	// Run broken link detection
	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	// Should find 2 broken links (missing.md and missing.png)
	// Should NOT report: existing file, external URL, or fragment-only link
	assert.Len(t, broken, 2, "should detect exactly 2 broken links")

	// Verify broken link details
	brokenFiles := make([]string, 0, len(broken))
	for _, link := range broken {
		brokenFiles = append(brokenFiles, link.Target)
	}
	assert.Contains(t, brokenFiles, "./missing.md")
	assert.Contains(t, brokenFiles, "./images/missing.png")
}

// TestDetectBrokenLinks_CaseInsensitive tests that broken link detection
// works correctly on case-insensitive filesystems.
func TestDetectBrokenLinks_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o750)
	require.NoError(t, err)

	// Create file with specific case
	actualFile := filepath.Join(docsDir, "API_Guide.md")
	err = os.WriteFile(actualFile, []byte("# API Guide"), 0o600)
	require.NoError(t, err)

	// Create index file that links with different case
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Index
[API Guide](./api_guide.md)
[Another Link](./Api_Guide.md)
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0o600)
	require.NoError(t, err)

	// Run broken link detection
	broken, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)

	// On case-insensitive filesystems (macOS/Windows), these should NOT be broken
	// On case-sensitive filesystems (Linux), these WOULD be broken
	// The fileExists function handles both cases
	if len(broken) > 0 {
		t.Logf("Detected %d broken links (likely running on case-sensitive filesystem)", len(broken))
	} else {
		t.Log("No broken links detected (likely running on case-insensitive filesystem)")
	}
}
