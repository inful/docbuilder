package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A file that breaks two rules of the name is renamed twice: the second
// rename used the path of the file before the first rename, and the
// fixer failed on a file it had already renamed.
func TestAFileThatBreaksTwoRulesOfTheNameIsRenamedOnce(t *testing.T) {
	dir := t.TempDir()
	adrDir := filepath.Join(dir, "adr")
	require.NoError(t, os.MkdirAll(adrDir, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(adrDir, "ADR Some Decision.md"), []byte("# Decision\n"), 0o600))

	fixer := NewFixer(NewLinter(&Config{}), false, false)
	result, err := fixer.Fix(dir)
	require.NoError(t, err)
	require.Empty(t, result.Errors, "the fixer failed on a file that breaks two rules of the name")

	entries, err := os.ReadDir(adrDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Regexp(t, `^adr-[0-9]{3}-[a-z0-9-]*\.md$`, entries[0].Name(),
		"the name of the file does not carry the number of the sequence")
}
