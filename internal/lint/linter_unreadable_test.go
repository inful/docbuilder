package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A file that cannot be read is an error: the lint reported success
// and the documentation passed the check.
func TestTheLintFailsWhenAFileCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "x.md")
	require.NoError(t, os.WriteFile(page, []byte("# X\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(page, 0o600) })

	linter := NewLinter(&Config{})

	_, err := linter.LintPath(dir)
	require.Error(t, err, "the lint reported success for a file it could not read")
}
