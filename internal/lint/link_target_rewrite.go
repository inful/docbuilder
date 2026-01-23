package lint

import (
	"fmt"
	"path/filepath"
	"strings"
)

// computeUpdatedLinkTarget computes the new link destination text when a target
// file has moved from oldAbs to newAbs.
//
// It must preserve:
// - link style (site-absolute vs relative)
// - extension style ("foo" vs "foo.md")
// - fragments ("#...").
func computeUpdatedLinkTarget(sourceFile string, originalTarget string, oldAbs string, newAbs string) (newTarget string, changed bool, err error) {
	_ = oldAbs // validation happens at call sites (broken link resolution)

	if originalTarget == "" {
		return "", false, nil
	}
	if strings.HasPrefix(originalTarget, "#") {
		return originalTarget, false, nil
	}

	pathPart, fragment := splitFragment(originalTarget)
	if pathPart == "" {
		return originalTarget, false, nil
	}
	if hasURLScheme(pathPart) {
		return originalTarget, false, nil
	}

	hasMarkdownExt := hasKnownMarkdownExtension(pathPart)
	wantsDotSlash := strings.HasPrefix(pathPart, "./")
	isSiteAbsolute := strings.HasPrefix(pathPart, "/")

	updatedPath, err := computeUpdatedLinkPath(sourceFile, newAbs, isSiteAbsolute, wantsDotSlash)
	if err != nil {
		return "", false, err
	}

	if !hasMarkdownExt {
		updatedPath = stripKnownMarkdownExtension(updatedPath)
	}

	newTarget = updatedPath + fragment
	return newTarget, newTarget != originalTarget, nil
}

func computeUpdatedLinkPath(sourceFile string, newAbs string, isSiteAbsolute bool, wantsDotSlash bool) (string, error) {
	if isSiteAbsolute {
		contentRoot := findContentRoot(sourceFile)
		if contentRoot == "" {
			return "", fmt.Errorf("failed to compute site-absolute link: content root not found for %q", sourceFile)
		}
		rel, err := filepath.Rel(contentRoot, newAbs)
		if err != nil {
			return "", fmt.Errorf("failed to compute site-absolute link relpath: %w", err)
		}
		return "/" + filepath.ToSlash(rel), nil
	}

	sourceDir := filepath.Dir(sourceFile)
	rel, err := filepath.Rel(sourceDir, newAbs)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative link relpath: %w", err)
	}
	updatedPath := filepath.ToSlash(rel)
	if wantsDotSlash && !strings.HasPrefix(updatedPath, "../") && !strings.HasPrefix(updatedPath, "./") {
		updatedPath = "./" + updatedPath
	}
	return updatedPath, nil
}

func splitFragment(target string) (pathPart string, fragment string) {
	idx := strings.Index(target, "#")
	if idx == -1 {
		return target, ""
	}
	return target[:idx], target[idx:]
}

func hasURLScheme(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:")
}

func hasKnownMarkdownExtension(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func stripKnownMarkdownExtension(target string) string {
	lower := strings.ToLower(target)
	if strings.HasSuffix(lower, ".md") {
		return target[:len(target)-len(".md")]
	}
	if strings.HasSuffix(lower, ".markdown") {
		return target[:len(target)-len(".markdown")]
	}
	return target
}
