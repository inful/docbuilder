package lint

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
//
// The paths are made absolute first: they come from different places,
// and one of them is relative to the working directory of the process.
// The comparison of a relative path with an absolute one never matches,
// and the link to a file that was renamed was never healed.
func pathsEqualCaseInsensitive(path1, path2 string) bool {
	if abs, err := filepath.Abs(path1); err == nil {
		path1 = abs
	}
	if abs, err := filepath.Abs(path2); err == nil {
		path2 = abs
	}

	// Normalize paths to forward slashes for consistent comparison
	path1 = filepath.ToSlash(filepath.Clean(path1))
	path2 = filepath.ToSlash(filepath.Clean(path2))

	// Case-insensitive comparison
	return strings.EqualFold(path1, path2)
}

// resolveRelativePath resolves a relative link path from a source file to an absolute path.
//
// The rootPath is the directory that was linted: the walk that looks
// for the file of a repository-root-relative link stops there.
func resolveRelativePath(sourceFile, linkTarget, rootPath string) (string, error) {
	// Remove any URL fragments (#section).
	targetPath, _, _ := strings.Cut(linkTarget, "#")

	// A link target is a URL: the space of a file name is written as
	// %20. The file on disk has the space, so the target is decoded
	// before it is looked for - otherwise every link to a file with a
	// space in its name is a broken link, and the fixer cannot heal it.
	if decoded, err := url.PathUnescape(targetPath); err == nil {
		targetPath = decoded
	}

	var resolvedPath string

	// Handle absolute paths (e.g., /local/docs/api-guide)
	// These are Hugo site-absolute paths that need to be resolved relative to content root
	if root, ok := strings.CutPrefix(targetPath, "/"); ok {
		// The link points at the root of the repository of the
		// documentation, so the file is looked for in the directory of
		// the source file and in the directories above it.
		if found, foundOK := resolveRepositoryRootPath(sourceFile, root, rootPath); foundOK {
			resolvedPath = found
		} else if contentRoot := findContentRoot(sourceFile); contentRoot != "" {
			// Join with the content root
			resolvedPath = filepath.Join(contentRoot, root)
		} else {
			// Fallback: treat as filesystem absolute path
			resolvedPath = "/" + root
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
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

// resolveRepositoryRootPath finds the file a repository-root-relative
// link points at.
//
// The link points at the root of the repository of the documentation, so
// the file is looked for in the directory of the source file and in the
// directories above it. The first one that holds the file is the one
// the link points at.
//
// The walk stops at the root of the documentation: a file above it is
// not published, and a link to it is broken. The walk stops at the root
// of the repository as well.
func resolveRepositoryRootPath(sourceFile, target, rootPath string) (string, bool) {
	dir := filepath.Dir(sourceFile)
	for {
		// The walk does not leave the documentation: the readme of the
		// repository is above it, and the site has no page for it.
		if rootPath != "" && !isInside(rootPath, dir) {
			return "", false
		}

		candidate := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(target, "/")))
		if fileExists(candidate) || fileExists(candidate+".md") || fileExists(candidate+".markdown") {
			return candidate, true
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// isInside reports whether a path is inside a directory.
func isInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
// skipping hidden directories and files matched by DefaultExcludes.
func collectMarkdownFiles(rootPath string, excludes []string) ([]string, error) {
	if excludes == nil {
		excludes = DefaultExcludes
	}
	globs := compileGlobs(excludes)
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

		// Skip git-forge-conventional files
		if isExcludedPath(path, rootPath, globs) {
			return nil
		}

		if IsDocFile(path) {
			filesToScan = append(filesToScan, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	return filesToScan, nil
}

// isExcludedPath reports whether path matches any compiled glob, relative
// to baseDir. Exposed as a package-level helper so non-Linter callers
// (such as the fixer walk) can reuse the same matching logic.
func isExcludedPath(path, baseDir string, globs []globPattern) bool {
	if len(globs) == 0 {
		return false
	}
	rel := path
	if baseDir != "" {
		if r, err := filepath.Rel(baseDir, path); err == nil {
			rel = r
		}
	}
	rel = filepath.ToSlash(rel)
	for _, g := range globs {
		if matchGlob(g, rel) {
			return true
		}
	}
	return false
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
