package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FileAssertions provides utilities for asserting file system state in tests.
type FileAssertions struct {
	t       *testing.T
	baseDir string
}

// NewFileAssertions creates a new file assertions helper.
func NewFileAssertions(t *testing.T, baseDir string) *FileAssertions {
	return &FileAssertions{
		t:       t,
		baseDir: baseDir,
	}
}

// AssertFileExists validates that a file exists.
func (fa *FileAssertions) AssertFileExists(relativePath string) *FileAssertions {
	fa.t.Helper()
	fullPath := filepath.Join(fa.baseDir, relativePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fa.t.Errorf("Expected file to exist: %s", fullPath)
	}
	return fa
}

// AssertDirExists validates that a directory exists.
func (fa *FileAssertions) AssertDirExists(relativePath string) *FileAssertions {
	fa.t.Helper()
	fullPath := filepath.Join(fa.baseDir, relativePath)
	if stat, err := os.Stat(fullPath); os.IsNotExist(err) {
		fa.t.Errorf("Expected directory to exist: %s", fullPath)
	} else if err == nil && !stat.IsDir() {
		fa.t.Errorf("Expected %s to be a directory, but it's a file", fullPath)
	}
	return fa
}

// AssertFileContains validates that a file contains expected content.
func (fa *FileAssertions) AssertFileContains(relativePath, expectedContent string) *FileAssertions {
	fa.t.Helper()
	fullPath := filepath.Join(fa.baseDir, relativePath)

	// #nosec G304 - test helper, paths are controlled by test code
	content, err := os.ReadFile(fullPath)
	if err != nil {
		fa.t.Errorf("Failed to read file %s: %v", fullPath, err)
		return fa
	}

	if !strings.Contains(string(content), expectedContent) {
		fa.t.Errorf("Expected file %s to contain %q\nActual content:\n%s",
			relativePath, expectedContent, string(content))
	}
	return fa
}

// AssertMinFileCount validates that a directory contains at least the expected number of files.
func (fa *FileAssertions) AssertMinFileCount(relativePath string, minCount int) *FileAssertions {
	fa.t.Helper()
	fullPath := filepath.Join(fa.baseDir, relativePath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		fa.t.Errorf("Failed to read directory %s: %v", fullPath, err)
		return fa
	}

	fileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			fileCount++
		}
	}

	if fileCount < minCount {
		fa.t.Errorf("Expected at least %d files in %s, found %d", minCount, relativePath, fileCount)
	}
	return fa
}
