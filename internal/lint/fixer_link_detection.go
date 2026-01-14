package lint

import (
	"os"
	"path/filepath"
	"strings"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// findLinksToFile finds all markdown links that reference the given target file.
// It searches from rootPath (typically the documentation root directory) to find
// all markdown files that might contain links to the target.
func (f *Fixer) findLinksToFile(targetPath, rootPath string) ([]LinkReference, error) {
	var links []LinkReference

	// Get absolute path of target for comparison
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to get absolute path for target").
			WithContext("target_path", targetPath).
			Build()
	}

	// Ensure rootPath is a directory
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to stat root path").
			WithContext("root_path", rootPath).
			Build()
	}

	searchRoot := rootPath
	if !rootInfo.IsDir() {
		searchRoot = filepath.Dir(rootPath)
	}

	err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process markdown files
		if info.IsDir() || !IsDocFile(path) {
			return nil
		}

		// Don't search the target file itself
		if path == targetPath {
			return nil
		}

		// Find links in this file
		fileLinks, err := f.findLinksInFile(path, absTarget)
		if err != nil {
			return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
				"failed to scan file for links").
				WithContext("file", path).
				WithContext("target", absTarget).
				Build()
		}

		links = append(links, fileLinks...)
		return nil
	})

	return links, err
}

// findLinksInFile scans a single markdown file for links to the target.
func (f *Fixer) findLinksInFile(sourceFile, targetPath string) ([]LinkReference, error) {
	// #nosec G304 -- sourceFile is from discovery walkFiles, not user input
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to read file").
			WithContext("file", sourceFile).
			Build()
	}

	var links []LinkReference
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Skip code blocks (simple heuristic: lines starting with spaces/tabs or in fenced blocks)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}

		// Find inline links: [text](path)
		inlineLinks := findInlineLinks(line, lineNum+1, sourceFile, targetPath)
		links = append(links, inlineLinks...)

		// Find reference-style links: [id]: path
		refLinks := findReferenceLinks(line, lineNum+1, sourceFile, targetPath)
		links = append(links, refLinks...)

		// Find image links: ![alt](path)
		imageLinks := findImageLinks(line, lineNum+1, sourceFile, targetPath)
		links = append(links, imageLinks...)
	}

	return links, nil
}

// inlineLinkInfo contains extracted inline link information.
type inlineLinkInfo struct {
	start  int
	end    int
	target string
}

// isInlineLinkStart checks if position i is the start of an inline link pattern ']('.
func isInlineLinkStart(line string, i int) bool {
	return i+1 < len(line) && line[i] == ']' && line[i+1] == '('
}

// extractInlineLink extracts link information from an inline link at position i.
func extractInlineLink(line string, i int) *inlineLinkInfo {
	start := findLinkTextStart(line, i)
	if start == -1 {
		return nil
	}

	end := findLinkEnd(line, i+2)
	if end == -1 {
		return nil
	}

	linkTarget := line[i+2 : end]

	// Skip external URLs
	if isExternalURL(linkTarget) {
		return nil
	}

	// Remove fragment for file existence check
	targetPath := strings.Split(linkTarget, "#")[0]
	if targetPath == "" {
		return nil // Fragment-only link (same page)
	}

	return &inlineLinkInfo{
		start:  start,
		end:    end,
		target: linkTarget,
	}
}

// findLinkTextStart finds the opening '[' bracket for link text, excluding image links.
func findLinkTextStart(line string, closeBracketPos int) int {
	for j := closeBracketPos - 1; j >= 0; j-- {
		if line[j] == '[' {
			// Make sure it's not an image link (preceded by !)
			if j > 0 && line[j-1] == '!' {
				return -1
			}
			return j
		}
	}
	return -1
}

// findLinkEnd finds the closing ')' parenthesis for the link target.
func findLinkEnd(line string, startPos int) int {
	end := strings.Index(line[startPos:], ")")
	if end == -1 {
		return -1
	}
	return startPos + end
}

// findInlineLinks finds inline-style markdown links: [text](path).
func findInlineLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	var links []LinkReference

	for i := range len(line) {
		if !isInlineLinkStart(line, i) {
			continue
		}

		linkInfo := extractInlineLink(line, i)
		if linkInfo == nil {
			continue
		}

		// Resolve the path
		resolved, err := resolveRelativePath(sourceFile, linkInfo.target)
		if err != nil {
			continue
		}

		// Check if this link points to our target
		if pathsEqualCaseInsensitive(resolved, targetPath) {
			linkRef := createLinkReference(line, lineNum, sourceFile, linkInfo)
			links = append(links, linkRef)
		}
	}

	return links
}

// createLinkReference creates a LinkReference from extracted link information.
func createLinkReference(line string, lineNum int, sourceFile string, linkInfo *inlineLinkInfo) LinkReference {
	// Extract fragment if present
	fragment := ""
	linkTarget := linkInfo.target
	if idx := strings.Index(linkTarget, "#"); idx != -1 {
		fragment = linkTarget[idx:]
		linkTarget = linkTarget[:idx]
	}

	return LinkReference{
		SourceFile: sourceFile,
		LineNumber: lineNum,
		LinkType:   LinkTypeInline,
		Target:     linkTarget,
		Fragment:   fragment,
		FullMatch:  line[linkInfo.start : linkInfo.end+1],
	}
}

// findReferenceLinks finds reference-style markdown links: [id]: path.
func findReferenceLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	var links []LinkReference

	// Pattern: [id]: path or [id]: path "title"
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return links
	}

	// Find closing ]
	endBracket := strings.Index(trimmed, "]:")
	if endBracket == -1 {
		return links
	}

	// Extract the path part (after ]:)
	rest := strings.TrimSpace(trimmed[endBracket+2:])
	if rest == "" {
		return links
	}

	// Remove optional title in quotes
	linkTarget := rest
	if idx := strings.Index(rest, " \""); idx != -1 {
		linkTarget = rest[:idx]
	} else if idx := strings.Index(rest, " '"); idx != -1 {
		linkTarget = rest[:idx]
	}

	linkTarget = strings.TrimSpace(linkTarget)

	// Skip external URLs
	if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
		return links
	}

	// Resolve the path
	resolved, err := resolveRelativePath(sourceFile, linkTarget)
	if err != nil {
		return links
	}

	// Check if this link points to our target (case-insensitive for filesystem compatibility)
	if pathsEqualCaseInsensitive(resolved, targetPath) {
		// Extract fragment if present
		fragment := ""
		if idx := strings.Index(linkTarget, "#"); idx != -1 {
			fragment = linkTarget[idx:]
			linkTarget = linkTarget[:idx]
		}

		links = append(links, LinkReference{
			SourceFile: sourceFile,
			LineNumber: lineNum,
			LinkType:   LinkTypeReference,
			Target:     linkTarget,
			Fragment:   fragment,
			FullMatch:  line,
		})
	}

	return links
}

// isImageLinkStart checks if position i is the start of an image link ![.
func isImageLinkStart(line string, i int) bool {
	return i+2 < len(line) && line[i] == '!' && line[i+1] == '['
}

// extractImageLink extracts image link information starting at position i.
// Returns nil if the image link is malformed or external.
func extractImageLink(line string, i int) *inlineLinkInfo {
	closeBracket := strings.Index(line[i+2:], "]")
	if closeBracket == -1 {
		return nil
	}
	closeBracket += i + 2

	if closeBracket+1 >= len(line) || line[closeBracket+1] != '(' {
		return nil
	}

	end := strings.Index(line[closeBracket+2:], ")")
	if end == -1 {
		return nil
	}
	end += closeBracket + 2

	linkTarget := line[closeBracket+2 : end]

	// Skip external URLs
	if isExternalURL(linkTarget) {
		return nil
	}

	return &inlineLinkInfo{
		start:  i,
		end:    end,
		target: linkTarget,
	}
}

// findImageLinks finds image markdown links: ![alt](path).
func findImageLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	var links []LinkReference

	// Look for ![]( pattern
	for i := range len(line) {
		if !isImageLinkStart(line, i) {
			continue
		}

		linkInfo := extractImageLink(line, i)
		if linkInfo == nil {
			continue
		}

		// Skip external URLs
		if strings.HasPrefix(linkInfo.target, "http://") || strings.HasPrefix(linkInfo.target, "https://") {
			continue
		}

		// Resolve the path
		resolved, err := resolveRelativePath(sourceFile, linkInfo.target)
		if err != nil {
			continue
		}

		// Check if this link points to our target (case-insensitive for filesystem compatibility)
		if pathsEqualCaseInsensitive(resolved, targetPath) {
			links = append(links, LinkReference{
				SourceFile: sourceFile,
				LineNumber: lineNum,
				LinkType:   LinkTypeImage,
				Target:     linkInfo.target,
				Fragment:   "",
				FullMatch:  line[linkInfo.start : linkInfo.end+1],
			})
		}
	}

	return links
}
