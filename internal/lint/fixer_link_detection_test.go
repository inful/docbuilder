package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkDiscovery_CaseInsensitive tests that link discovery works with
// case-insensitive path matching.
func TestLinkDiscovery_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o750)
	require.NoError(t, err)

	// Create target file with specific case
	targetFile := filepath.Join(docsDir, "API_Guide.md")
	err = os.WriteFile(targetFile, []byte("# API Guide"), 0o600)
	require.NoError(t, err)

	// Create index file that links to target with different case
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Index
[API Guide](./api_guide.md)
[Another Reference](./Api_Guide.md)
![Diagram](./API_GUIDE.md)
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0o600)
	require.NoError(t, err)

	// Find links to the target file
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	links, err := fixer.findLinksToFile(targetFile, tmpDir)
	require.NoError(t, err)

	// On case-insensitive comparison, all three links should be found
	// even though they have different cases
	assert.GreaterOrEqual(t, len(links), 3, "should find links with case-insensitive matching")
}
