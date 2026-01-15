package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestFixer_UpdatesFrontmatterFingerprint_SetsLastmodWhenMissingFingerprint(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("# Title\n\nHello\n"), 0o600))

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)
	fixer.nowFn = func() time.Time { return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) }

	res, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Fingerprints, 1)
	require.True(t, res.Fingerprints[0].Success)

	// #nosec G304 -- test reads a temp file path under t.TempDir().
	updatedBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	updatedStr := string(updatedBytes)

	ok, verr := mdfp.VerifyFingerprint(updatedStr)
	require.NoError(t, verr)
	require.True(t, ok)

	lastmod, ok := extractLastmodFromFrontmatter(updatedStr)
	require.True(t, ok)
	require.Equal(t, "2026-01-15", lastmod)
}

func TestFixer_UpdatesFrontmatterFingerprint_UpdatesLastmodWhenFingerprintChanges(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")

	seed, err := mdfp.ProcessContent("# Title\n\nHello\n")
	require.NoError(t, err)
	seed = setOrUpdateLastmodInFrontmatter(seed, "2000-01-01")

	// Change the body but keep the old fingerprint + lastmod (should trigger fix).
	mismatched := strings.Replace(seed, "Hello", "Hello changed", 1)
	require.NoError(t, os.WriteFile(path, []byte(mismatched), 0o600))

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)
	fixer.nowFn = func() time.Time { return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) }

	res, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Fingerprints, 1)
	require.True(t, res.Fingerprints[0].Success)

	// #nosec G304 -- test reads a temp file path under t.TempDir().
	updatedBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	updatedStr := string(updatedBytes)

	ok, verr := mdfp.VerifyFingerprint(updatedStr)
	require.NoError(t, verr)
	require.True(t, ok)

	lastmod, ok := extractLastmodFromFrontmatter(updatedStr)
	require.True(t, ok)
	require.Equal(t, "2026-01-15", lastmod)
}

func TestFixer_UpdatesFrontmatterFingerprint_DoesNotUpdateLastmodWhenFingerprintUnchanged(t *testing.T) {
tmpDir := t.TempDir()
path := filepath.Join(tmpDir, "doc.md")

// Create a file with valid fingerprint and lastmod
seed, err := mdfp.ProcessContent("# Title\n\nHello\n")
require.NoError(t, err)
seed = setOrUpdateLastmodInFrontmatter(seed, "2000-01-01")
require.NoError(t, os.WriteFile(path, []byte(seed), 0o600))

// Verify the file has valid fingerprint and correct lastmod
ok, verr := mdfp.VerifyFingerprint(seed)
require.NoError(t, verr)
require.True(t, ok)
lastmodBefore, ok := extractLastmodFromFrontmatter(seed)
require.True(t, ok)
require.Equal(t, "2000-01-01", lastmodBefore)

// Re-run the fixer without changing content
linter := NewLinter(&Config{Format: "text"})
fixer := NewFixer(linter, false, false)
fixer.nowFn = func() time.Time { return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) }

res, err := fixer.Fix(tmpDir)
require.NoError(t, err)
require.NotNil(t, res)

// #nosec G304 -- test reads a temp file path under t.TempDir().
updatedBytes, err := os.ReadFile(path)
require.NoError(t, err)
updatedStr := string(updatedBytes)

// Verify fingerprint is still valid
ok, verr = mdfp.VerifyFingerprint(updatedStr)
require.NoError(t, verr)
require.True(t, ok)

// CRITICAL: lastmod should remain unchanged because fingerprint didn't change
lastmodAfter, ok := extractLastmodFromFrontmatter(updatedStr)
require.True(t, ok)
require.Equal(t, "2000-01-01", lastmodAfter, "lastmod should not be updated when fingerprint is unchanged")
}
