package lint

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRenameMappings_RequiresAbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()
	docsRoot := filepath.Join(tmpDir, "docs")

	mappings := []RenameMapping{{
		OldAbs: "docs/old.md",
		NewAbs: filepath.Join(docsRoot, "new.md"),
		Source: RenameSourceFixer,
	}}

	_, err := NormalizeRenameMappings(mappings, []string{docsRoot})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func TestNormalizeRenameMappings_FiltersToDocsRoots(t *testing.T) {
	tmpDir := t.TempDir()
	docsRoot := filepath.Join(tmpDir, "docs")
	otherRoot := filepath.Join(tmpDir, "other")

	mappings := []RenameMapping{
		{
			OldAbs: filepath.Join(docsRoot, "old.md"),
			NewAbs: filepath.Join(docsRoot, "new.md"),
			Source: RenameSourceFixer,
		},
		{
			OldAbs: filepath.Join(otherRoot, "old.md"),
			NewAbs: filepath.Join(otherRoot, "new.md"),
			Source: RenameSourceFixer,
		},
		{
			// Mixed roots should be dropped as out-of-scope.
			OldAbs: filepath.Join(docsRoot, "a.md"),
			NewAbs: filepath.Join(otherRoot, "a.md"),
			Source: RenameSourceFixer,
		},
	}

	got, err := NormalizeRenameMappings(mappings, []string{docsRoot})
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, filepath.Join(docsRoot, "old.md"), got[0].OldAbs)
	assert.Equal(t, filepath.Join(docsRoot, "new.md"), got[0].NewAbs)
}

func TestNormalizeRenameMappings_DedupesAndSortsDeterministically(t *testing.T) {
	tmpDir := t.TempDir()
	docsRoot := filepath.Join(tmpDir, "docs")

	aOld := filepath.Join(docsRoot, "a.md")
	aNew := filepath.Join(docsRoot, "a-new.md")
	bOld := filepath.Join(docsRoot, "b.md")
	bNew := filepath.Join(docsRoot, "b-new.md")

	mappings := []RenameMapping{
		{OldAbs: bOld, NewAbs: bNew, Source: RenameSourceFixer},
		{OldAbs: aOld, NewAbs: aNew, Source: RenameSourceFixer},
		// Duplicate mapping should be removed.
		{OldAbs: aOld, NewAbs: aNew, Source: RenameSourceFixer},
	}

	got, err := NormalizeRenameMappings(mappings, []string{docsRoot})
	require.NoError(t, err)

	require.Len(t, got, 2)
	assert.Equal(t, aOld, got[0].OldAbs)
	assert.Equal(t, bOld, got[1].OldAbs)
}
