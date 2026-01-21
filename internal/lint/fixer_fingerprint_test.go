package lint

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/inful/mdfp"
	"github.com/stretchr/testify/require"
)

func mustExtractFrontmatterLastmod(t *testing.T, content string) (string, bool) {
	t.Helper()

	fmRaw, _, had, _, err := frontmatter.Split([]byte(content))
	require.NoError(t, err)
	require.True(t, had)

	fields, err := frontmatter.ParseYAML(fmRaw)
	require.NoError(t, err)

	val, ok := fields["lastmod"]
	if !ok {
		return "", false
	}

	switch v := val.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return "", false
		}
		return v, true
	case time.Time:
		return v.UTC().Format("2006-01-02"), true
	default:
		return "", false
	}
}

func buildDocWithFingerprint(t *testing.T, fields map[string]any, body string) string {
	t.Helper()

	hashStyle := frontmatter.Style{Newline: "\n"}

	fieldsForHash := make(map[string]any, len(fields))
	for k, v := range fields {
		if k == mdfp.FingerprintField {
			continue
		}
		if k == "lastmod" {
			continue
		}
		if k == "uid" {
			continue
		}
		if k == "aliases" {
			continue
		}
		fieldsForHash[k] = v
	}

	frontmatterForHash := ""
	if len(fieldsForHash) > 0 {
		serialized, err := frontmatter.SerializeYAML(fieldsForHash, hashStyle)
		require.NoError(t, err)
		frontmatterForHash = strings.TrimSuffix(string(serialized), "\n")
	}

	withFingerprint := make(map[string]any, len(fields)+1)
	maps.Copy(withFingerprint, fields)
	withFingerprint[mdfp.FingerprintField] = mdfp.CalculateFingerprintFromParts(frontmatterForHash, body)

	fmBytes, err := frontmatter.SerializeYAML(withFingerprint, hashStyle)
	require.NoError(t, err)
	content := frontmatter.Join(fmBytes, []byte(body), true, frontmatter.Style{Newline: "\n"})
	return string(content)
}

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

	rule := &FrontmatterFingerprintRule{}
	issues, checkErr := rule.Check(path)
	require.NoError(t, checkErr)
	require.Empty(t, issues)
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

	rule := &FrontmatterFingerprintRule{}
	issues, checkErr := rule.Check(path)
	require.NoError(t, checkErr)
	require.Empty(t, issues)

	lastmod, ok := mustExtractFrontmatterLastmod(t, updatedStr)
	require.True(t, ok)
	require.Equal(t, "2026-01-15", lastmod)
}

func TestFixer_UpdatesFrontmatterFingerprint_UpdatesLastmodWhenFingerprintChanges(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")

	seed := buildDocWithFingerprint(t, map[string]any{
		"title":   "Title",
		"lastmod": "2000-01-01",
	}, "# Title\n\nHello\n")

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

	rule := &FrontmatterFingerprintRule{}
	issues, checkErr := rule.Check(path)
	require.NoError(t, checkErr)
	require.Empty(t, issues)

	lastmod, ok := mustExtractFrontmatterLastmod(t, updatedStr)
	require.True(t, ok)
	require.Equal(t, "2026-01-15", lastmod)
}

func TestFixer_UpdatesFrontmatterFingerprint_DoesNotUpdateLastmodWhenFingerprintUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")

	// Create a file with valid fingerprint and lastmod
	seed := buildDocWithFingerprint(t, map[string]any{
		"title":   "Title",
		"lastmod": "2000-01-01",
	}, "# Title\n\nHello\n")
	require.NoError(t, os.WriteFile(path, []byte(seed), 0o600))

	// Verify the file has valid fingerprint and correct lastmod
	rule := &FrontmatterFingerprintRule{}
	issues, checkErr := rule.Check(path)
	require.NoError(t, checkErr)
	require.Empty(t, issues)
	lastmodBefore, ok := mustExtractFrontmatterLastmod(t, seed)
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
	issues, checkErr = rule.Check(path)
	require.NoError(t, checkErr)
	require.Empty(t, issues)

	// CRITICAL: lastmod should remain unchanged because fingerprint didn't change
	lastmodAfter, ok := mustExtractFrontmatterLastmod(t, updatedStr)
	require.True(t, ok)
	require.Equal(t, "2000-01-01", lastmodAfter, "lastmod should not be updated when fingerprint is unchanged")
}
