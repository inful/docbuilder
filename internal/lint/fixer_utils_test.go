package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathsEqualCaseInsensitive tests case-insensitive path comparison.
func TestPathsEqualCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected bool
	}{
		{
			name:     "exact match",
			path1:    "/docs/api-guide.md",
			path2:    "/docs/api-guide.md",
			expected: true,
		},
		{
			name:     "case difference",
			path1:    "/docs/API_Guide.md",
			path2:    "/docs/api_guide.md",
			expected: true,
		},
		{
			name:     "mixed case in directory",
			path1:    "/Docs/API/Guide.md",
			path2:    "/docs/api/guide.md",
			expected: true,
		},
		{
			name:     "different files",
			path1:    "/docs/guide.md",
			path2:    "/docs/tutorial.md",
			expected: false,
		},
		{
			name:     "normalized paths",
			path1:    "/docs/../docs/guide.md",
			path2:    "/docs/guide.md",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathsEqualCaseInsensitive(tt.path1, tt.path2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFileExists tests file existence checking with case-insensitive fallback.
func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "TestFile.md")
	err := os.WriteFile(testFile, []byte("test"), 0o600)
	require.NoError(t, err)

	t.Run("exact case match", func(t *testing.T) {
		exists := fileExists(testFile)
		assert.True(t, exists)
	})

	t.Run("different case", func(t *testing.T) {
		lowerCasePath := filepath.Join(tmpDir, "testfile.md")
		exists := fileExists(lowerCasePath)
		assert.True(t, exists, "should find file with case-insensitive lookup")
	})

	t.Run("non-existent file", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "missing.md")
		exists := fileExists(nonExistent)
		assert.False(t, exists)
	})
}
