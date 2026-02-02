// Package templates provides functionality for discovering, parsing, and instantiating
// documentation templates from rendered documentation sites.
//
// Templates are discovered from a documentation site's taxonomy page, parsed from HTML
// with metadata in meta tags, and rendered using Go's text/template engine with user
// inputs and sequence helpers.
package templates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// WriteGeneratedFile writes the generated content to a file under the docs directory.
//
// The function ensures:
//   - The output path is relative to docsDir (no path traversal)
//   - Parent directories are created if needed
//   - Existing files are never overwritten (returns error if file exists)
//   - File permissions are set to 0o600 (read/write for owner only)
//
// Parameters:
//   - docsDir: The base documentation directory (typically "docs/")
//   - relativePath: Path relative to docsDir (e.g., "adr/adr-001-title.md")
//   - content: The markdown content to write
//
// Returns:
//   - The full path of the written file
//   - An error if the file already exists, path is invalid, or write fails
//
// Example:
//
//	fullPath, err := WriteGeneratedFile("docs", "adr/adr-001.md", "# My ADR\n")
//	if err != nil {
//	    // Handle error (e.g., file already exists)
//	}
func WriteGeneratedFile(docsDir, relativePath, content string) (string, error) {
	if docsDir == "" {
		return "", errors.New("docs directory is required")
	}
	if relativePath == "" {
		return "", errors.New("output path is required")
	}

	cleanRel := filepath.Clean(relativePath)
	if filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, "..") {
		return "", errors.New("output path must be relative to docs")
	}

	fullPath := filepath.Join(docsDir, cleanRel)
	rel, err := filepath.Rel(docsDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("output path escapes docs directory")
	}

	if err = os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// #nosec G304 -- fullPath is validated to stay under docsDir.
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		// Check if error is due to file already existing
		if errors.Is(err, os.ErrExist) || errors.Is(err, syscall.EEXIST) {
			return "", fmt.Errorf("file already exists: %s", fullPath)
		}
		return "", fmt.Errorf("write output file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.WriteString(content); err != nil {
		return "", fmt.Errorf("write output file: %w", err)
	}

	return fullPath, nil
}
