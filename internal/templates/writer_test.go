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

func TestWriteGeneratedFile_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Create existing file
	existingPath := filepath.Join(docsDir, "adr", "adr-001.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(existingPath), 0o750))
	require.NoError(t, os.WriteFile(existingPath, []byte("existing content"), 0o600))

	// Try to write to same path
	_, err := WriteGeneratedFile(docsDir, "adr/adr-001.md", "new content")
	require.Error(t, err)
	require.Contains(t, err.Error(), "file already exists")

	// Verify original file unchanged
	// #nosec G304 -- existingPath is controlled by test.
	data, err := os.ReadFile(existingPath)
	require.NoError(t, err)
	require.Equal(t, "existing content", string(data))
}
