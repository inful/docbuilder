package lint

import (
	"fmt"
	"os"
	"strings"
)

// detectBrokenLinks scans all markdown files in a path for links to non-existent files.
func detectBrokenLinks(rootPath string) ([]BrokenLink, error) {
	var brokenLinks []BrokenLink

	// Determine if rootPath is a file or directory
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	var filesToScan []string
	if info.IsDir() {
		filesToScan, err = collectMarkdownFiles(rootPath)
		if err != nil {
			return nil, err
		}
	} else if IsDocFile(rootPath) {
		filesToScan = []string{rootPath}
	}

	// Scan each file for broken links
	for _, file := range filesToScan {
		broken, err := detectBrokenLinksInFile(file)
		if err != nil {
			// Continue with other files even if one fails
			continue
		}
		brokenLinks = append(brokenLinks, broken...)
	}

	return brokenLinks, nil
}

// detectBrokenLinksInFile scans a single markdown file for broken links.
func detectBrokenLinksInFile(sourceFile string) ([]BrokenLink, error) {
	// #nosec G304 -- sourceFile is from discovery walkFiles, not user input
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var brokenLinks []BrokenLink
	lines := strings.Split(string(content), "\n")

	inCodeBlock := false
	for lineNum, line := range lines {
		// Track code block boundaries
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Skip lines inside code blocks or indented code blocks
		if inCodeBlock || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}

		// Check inline links
		broken := checkInlineLinksBroken(line, lineNum+1, sourceFile)
		brokenLinks = append(brokenLinks, broken...)

		// Check reference-style links
		brokenRef := checkReferenceLinksBroken(line, lineNum+1, sourceFile)
		brokenLinks = append(brokenLinks, brokenRef...)

		// Check image links
		brokenImg := checkImageLinksBroken(line, lineNum+1, sourceFile)
		brokenLinks = append(brokenLinks, brokenImg...)
	}

	return brokenLinks, nil
}

// isBrokenLink checks if a link target points to a non-existent file.
func isBrokenLink(sourceFile, linkTarget string) bool {
	resolved, err := resolveRelativePath(sourceFile, linkTarget)
	if err != nil {
		return false
	}
	return !fileExists(resolved)
}

// checkInlineLinksBroken checks for broken inline links in a line.
func checkInlineLinksBroken(line string, lineNum int, sourceFile string) []BrokenLink {
	var broken []BrokenLink

	for i := range len(line) {
		if !isInlineLinkStart(line, i) {
			continue
		}

		// Skip if this link is inside inline code
		if isInsideInlineCode(line, i) {
			continue
		}

		linkInfo := extractInlineLink(line, i)
		if linkInfo == nil {
			continue
		}

		if isHugoShortcodeLinkTarget(linkInfo.target) {
			continue
		}

		if isUIDAliasLinkTarget(linkInfo.target) {
			continue
		}

		if isBrokenLink(sourceFile, linkInfo.target) {
			broken = append(broken, BrokenLink{
				SourceFile: sourceFile,
				LineNumber: lineNum,
				Target:     linkInfo.target,
				LinkType:   LinkTypeInline,
			})
		}
	}

	return broken
}

// checkReferenceLinksBroken checks for broken reference-style links in a line.
func checkReferenceLinksBroken(line string, lineNum int, sourceFile string) []BrokenLink {
	var broken []BrokenLink

	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return broken
	}

	// Skip if the entire line is inside inline code
	if isInsideInlineCode(line, 0) {
		return broken
	}

	_, after, ok := strings.Cut(trimmed, "]:")
	if !ok {
		return broken
	}

	rest := strings.TrimSpace(after)
	if rest == "" {
		return broken
	}

	linkTarget := rest
	if before, _, ok := strings.Cut(rest, " \""); ok {
		linkTarget = before
	} else if before, _, ok := strings.Cut(rest, " '"); ok {
		linkTarget = before
	}
	linkTarget = strings.TrimSpace(linkTarget)
	if isHugoShortcodeLinkTarget(linkTarget) {
		return broken
	}

	if isUIDAliasLinkTarget(linkTarget) {
		return broken
	}

	// Skip external URLs
	if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
		return broken
	}

	// Remove fragment for file existence check
	targetPath := strings.Split(linkTarget, "#")[0]
	if targetPath == "" {
		return broken
	}

	// Resolve and check if file exists
	resolved, err := resolveRelativePath(sourceFile, targetPath)
	if err != nil {
		return broken
	}

	if !fileExists(resolved) {
		broken = append(broken, BrokenLink{
			SourceFile: sourceFile,
			LineNumber: lineNum,
			Target:     linkTarget,
			LinkType:   LinkTypeReference,
		})
	}

	return broken
}

// isHugoShortcodeLinkTarget reports whether the link target is a Hugo shortcode
// reference (starting with `{{%` or `{{<`).
func isHugoShortcodeLinkTarget(linkTarget string) bool {
	trim := strings.TrimSpace(linkTarget)
	return strings.HasPrefix(trim, "{{%") || strings.HasPrefix(trim, "{{<")
}

// isUIDAliasLinkTarget reports whether linkTarget is a UID alias path (starting with "/_uid/").
func isUIDAliasLinkTarget(linkTarget string) bool {
	trim := strings.TrimSpace(linkTarget)
	return strings.HasPrefix(trim, "/_uid/")
}

// checkImageLinksBroken checks for broken image links in a line.
func checkImageLinksBroken(line string, lineNum int, sourceFile string) []BrokenLink {
	var broken []BrokenLink

	for i := range len(line) {
		if !isImageLinkStart(line, i) {
			continue
		}
		if isInsideInlineCode(line, i) {
			continue
		}

		linkInfo := extractImageLink(line, i)
		if linkInfo == nil {
			continue
		}

		if isBrokenLink(sourceFile, linkInfo.target) {
			broken = append(broken, BrokenLink{
				SourceFile: sourceFile,
				LineNumber: lineNum,
				Target:     linkInfo.target,
				LinkType:   LinkTypeImage,
			})
		}
	}
	return broken
}
