package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeUpdatedLinkTarget_RelativeAcrossDirectories(t *testing.T) {
	repoRoot := t.TempDir()
	sourceFile := filepath.Join(repoRoot, "docs", "a", "source.md")
	oldAbs := filepath.Join(repoRoot, "docs", "old", "target.md")
	newAbs := filepath.Join(repoRoot, "docs", "new", "target.md")

	writeFile(t, sourceFile, "# source")
	writeFile(t, oldAbs, "# old")
	writeFile(t, newAbs, "# new")

	originalTarget := "../old/target.md#section"
	updated, changed, err := computeUpdatedLinkTarget(sourceFile, originalTarget, oldAbs, newAbs)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "../new/target.md#section", updated)

	resolved, err := resolveRelativePath(sourceFile, updated)
	require.NoError(t, err)
	require.Equal(t, newAbs, resolved)
}

func TestComputeUpdatedLinkTarget_SameDir_PreservesDotSlash(t *testing.T) {
	repoRoot := t.TempDir()
	sourceFile := filepath.Join(repoRoot, "docs", "a", "source.md")
	oldAbs := filepath.Join(repoRoot, "docs", "a", "old.md")
	newAbs := filepath.Join(repoRoot, "docs", "a", "new.md")

	writeFile(t, sourceFile, "# source")
	writeFile(t, oldAbs, "# old")
	writeFile(t, newAbs, "# new")

	updated, changed, err := computeUpdatedLinkTarget(sourceFile, "./old.md", oldAbs, newAbs)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "./new.md", updated)

	resolved, err := resolveRelativePath(sourceFile, updated)
	require.NoError(t, err)
	require.Equal(t, newAbs, resolved)
}

func TestComputeUpdatedLinkTarget_SiteAbsolute_PreservesLeadingSlash(t *testing.T) {
	repoRoot := t.TempDir()
	sourceFile := filepath.Join(repoRoot, "content", "en", "guide", "source.md")
	oldAbs := filepath.Join(repoRoot, "content", "en", "api", "old.md")
	newAbs := filepath.Join(repoRoot, "content", "en", "api", "new.md")

	writeFile(t, sourceFile, "# source")
	writeFile(t, oldAbs, "# old")
	writeFile(t, newAbs, "# new")

	updated, changed, err := computeUpdatedLinkTarget(sourceFile, "/en/api/old.md", oldAbs, newAbs)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "/en/api/new.md", updated)

	resolved, err := resolveRelativePath(sourceFile, updated)
	require.NoError(t, err)
	require.Equal(t, newAbs, resolved)
}

func TestComputeUpdatedLinkTarget_Extensionless_PreservesNoExtension(t *testing.T) {
	repoRoot := t.TempDir()
	sourceFile := filepath.Join(repoRoot, "content", "en", "guide", "source.md")
	oldAbs := filepath.Join(repoRoot, "content", "en", "api", "old.md")
	newAbs := filepath.Join(repoRoot, "content", "en", "api", "new.md")

	writeFile(t, sourceFile, "# source")
	writeFile(t, oldAbs, "# old")
	writeFile(t, newAbs, "# new")

	updated, changed, err := computeUpdatedLinkTarget(sourceFile, "/en/api/old", oldAbs, newAbs)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "/en/api/new", updated)

	resolved, err := resolveRelativePath(sourceFile, updated)
	require.NoError(t, err)
	require.Equal(t, newAbs, resolved)
}

func TestComputeUpdatedLinkTarget_FragmentOnly_NoChange(t *testing.T) {
	repoRoot := t.TempDir()
	sourceFile := filepath.Join(repoRoot, "docs", "a", "source.md")
	oldAbs := filepath.Join(repoRoot, "docs", "a", "old.md")
	newAbs := filepath.Join(repoRoot, "docs", "a", "new.md")

	writeFile(t, sourceFile, "# source")
	writeFile(t, oldAbs, "# old")
	writeFile(t, newAbs, "# new")

	updated, changed, err := computeUpdatedLinkTarget(sourceFile, "#section", oldAbs, newAbs)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, "#section", updated)
}

func writeFile(t *testing.T, absPath string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o750))
	require.NoError(t, os.WriteFile(absPath, []byte(content), 0o600))
}
