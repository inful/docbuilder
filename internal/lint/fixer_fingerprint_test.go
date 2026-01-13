package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inful/mdfp"
	"github.com/stretchr/testify/require"
)

func TestFixer_UpdatesFrontmatterFingerprint(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("# Title\n\nHello\n"), 0o600))

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	res, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Fingerprints, 1)
	require.True(t, res.Fingerprints[0].Success)

	// #nosec G304 -- test reads a temp file path under t.TempDir().
	updatedBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	ok, verr := mdfp.VerifyFingerprint(string(updatedBytes))
	require.NoError(t, verr)
	require.True(t, ok)
}

func TestFixer_DryRun_DoesNotWriteFingerprintChanges(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")
	original := "# Title\n\nHello\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, true, false) // dry-run

	res, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Fingerprints, 1)

	// #nosec G304 -- test reads a temp file path under t.TempDir().
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, string(after))
}
