package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeNextInSequence(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "adr"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "adr", "adr-001-first.md"), []byte("# one"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "adr", "adr-010-second.md"), []byte("# two"), 0o600))

	def := SequenceDefinition{
		Name:  "adr",
		Dir:   "adr",
		Glob:  "adr-*.md",
		Regex: "^adr-(\\d{3})-",
		Start: 1,
	}

	next, err := ComputeNextInSequence(def, docsDir)
	require.NoError(t, err)
	require.Equal(t, 11, next)
}

func TestComputeNextInSequence_UsesStartWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "adr"), 0o750))

	def := SequenceDefinition{
		Name:  "adr",
		Dir:   "adr",
		Glob:  "adr-*.md",
		Regex: "^adr-(\\d{3})-",
		Start: 5,
	}

	next, err := ComputeNextInSequence(def, docsDir)
	require.NoError(t, err)
	require.Equal(t, 5, next)
}

func TestComputeNextInSequence_InvalidDir(t *testing.T) {
	def := SequenceDefinition{
		Name:  "adr",
		Dir:   "../adr",
		Glob:  "adr-*.md",
		Regex: "^adr-(\\d{3})-",
		Start: 1,
	}

	_, err := ComputeNextInSequence(def, "/tmp/docs")
	require.Error(t, err)
}
