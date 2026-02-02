package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteGeneratedFile(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	fullPath, err := WriteGeneratedFile(docsDir, "adr/adr-001.md", "content")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(docsDir, "adr", "adr-001.md"), fullPath)

	// #nosec G304 -- fullPath is controlled by test.
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	require.Equal(t, "content", string(data))
}

func TestWriteGeneratedFile_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	_, err := WriteGeneratedFile(docsDir, "../outside.md", "content")
	require.Error(t, err)
}
