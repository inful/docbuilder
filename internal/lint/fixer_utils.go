package lint

import (
	"os"
	"path/filepath"
	"strings"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// fileExists checks if a file exists (case-insensitive on applicable filesystems).
func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}

	// On case-insensitive filesystems, try case-insensitive lookup
	// by checking the directory listing
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), base) {
			return true
		}
	}

	return false
}

// pathsEqualCaseInsensitive compares two paths case-insensitively.
// This is important for filesystems like macOS (HFS+/APFS) and Windows (NTFS)
// that are case-insensitive but case-preserving.
func pathsEqualCaseInsensitive(path1, path2 string) bool {
	// Normalize paths to forward slashes for consistent comparison
	path1 = filepath.ToSlash(filepath.Clean(path1))
	path2 = filepath.ToSlash(filepath.Clean(path2))

	// Case-insensitive comparison
	return strings.EqualFold(path1, path2)
}

// resolveRelativePath resolves a relative link path from a source file to an absolute path.
func resolveRelativePath(sourceFile, linkTarget string) (string, error) {
	// Remove any URL fragments (#section)
	targetPath := strings.Split(linkTarget, "#")[0]

	var resolvedPath string

	// Handle absolute paths (e.g., /local/docs/api-guide)
	// These are Hugo site-absolute paths that need to be resolved relative to content root
	if strings.HasPrefix(targetPath, "/") {
		// Find the content root by walking up from source file
		contentRoot := findContentRoot(sourceFile)
		if contentRoot != "" {
			// Strip leading slash and join with content root
			targetPath = strings.TrimPrefix(targetPath, "/")
			resolvedPath = filepath.Join(contentRoot, targetPath)
		} else {
			// Fallback: treat as filesystem absolute path
			resolvedPath = targetPath
		}
	} else {
		// Relative path - resolve relative to source file directory
		sourceDir := filepath.Dir(sourceFile)
		resolvedPath = filepath.Join(sourceDir, targetPath)
	}

	cleanPath := filepath.Clean(resolvedPath)

	// Try with .md extension if file doesn't exist as-is
	if !fileExists(cleanPath) {
		// Try adding .md extension (Hugo strips .md from URLs)
		withMd := cleanPath + ".md"
		if fileExists(withMd) {
			return withMd, nil
		}
		// Try adding .markdown extension
		withMarkdown := cleanPath + ".markdown"
		if fileExists(withMarkdown) {
			return withMarkdown, nil
		}
	}

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to resolve absolute path").
			WithContext("path", cleanPath).
			Build()
	}

	return absPath, nil
}

// findContentRoot finds the content directory by walking up from the source file.
// It looks for a directory named "content" in the path hierarchy.
func findContentRoot(sourceFile string) string {
	dir := filepath.Dir(sourceFile)
	for {
		if filepath.Base(dir) == "content" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding content directory
			return ""
		}
		dir = parent
	}
}

// collectMarkdownFiles walks a directory tree and returns all markdown files,
// skipping hidden directories and ignored files.
func collectMarkdownFiles(rootPath string) ([]string, error) {
	var filesToScan []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files
		if info.Name() != "." && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Skip standard ignored files (case-insensitive)
		if isIgnoredFile(info.Name()) {
			return nil
		}

		if IsDocFile(path) {
			filesToScan = append(filesToScan, path)
		}
		return nil
	})
	if err != nil {
		return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to walk directory").
			WithContext("root_path", rootPath).
			Build()
	}
	return filesToScan, nil
}

// isExternalURL checks if a link target is an external URL.
func isExternalURL(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

// isInsideInlineCode checks if a position in a line is inside inline code (backticks).
func isInsideInlineCode(line string, pos int) bool {
	backtickCount := 0
	for i := 0; i < pos && i < len(line); i++ {
		if line[i] == '`' {
			backtickCount++
		}
	}
	// If odd number of backticks before position, we're inside inline code
	return backtickCount%2 == 1
}
