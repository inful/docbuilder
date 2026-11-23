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

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
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

	// The links of the path are followed: a directory of the docs that
	// is a link to another place would take the file outside of the
	// docs directory.
	resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(fullPath))
	if err != nil {
		// The directory does not exist yet: the links of the parents are
		// followed instead.
		resolvedDir, err = resolveExistingDir(filepath.Dir(fullPath))
		if err != nil {
			return "", fmt.Errorf("resolve the output directory: %w", err)
		}
	}
	resolvedDocs, err := filepath.EvalSymlinks(docsDir)
	if err != nil {
		resolvedDocs = docsDir
	}
	resolvedRel, err := filepath.Rel(resolvedDocs, filepath.Join(resolvedDir, filepath.Base(fullPath)))
	if err != nil || strings.HasPrefix(resolvedRel, "..") {
		return "", errors.New("output path escapes docs directory")
	}

	if err = os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// The file has to be new: the old way refused to write over a file
	// that was there.
	if _, statErr := os.Stat(fullPath); statErr == nil {
		return "", fmt.Errorf("file already exists: %s", fullPath)
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("write output file: %w", statErr)
	}

	// The file is written without a window in which it is half written:
	// a full disk or a killed process left a truncated file behind, and
	// every later write to that path failed with "file already exists".
	if err := foundation.WriteFileAtomic(fullPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write output file: %w", err)
	}

	return fullPath, nil
}

// resolveExistingDir resolves the links of the deepest directory of the
// path that exists.
func resolveExistingDir(path string) (string, error) {
	current := path
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return resolved, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		current = parent
	}
}
