package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The dry run reports what would happen: nothing was written, and the
// report said that the fingerprints were updated.
func TestTheDryRunSaysWhatWouldHappen(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "Bad Name.md")
	require.NoError(t, os.WriteFile(page, []byte("# Bad\n"), 0o600))

	fixer := NewFixer(NewLinter(&Config{}), true, false)
	result, err := fixer.Fix(dir)
	require.NoError(t, err)

	summary := result.Summary()
	require.Contains(t, summary, "Fingerprints to update")
	require.NotContains(t, summary, "Fingerprints updated")
	require.Contains(t, summary, "Files to be renamed")
	require.NotContains(t, summary, "Files renamed")
}
