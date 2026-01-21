package lint

import (
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
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

	body := content
	fmRaw, fmBody, had, style, splitErr := frontmatter.Split(content)
	_ = fmRaw
	_ = had
	_ = style
	if splitErr == nil {
		body = fmBody
	}

	links, parseErr := markdown.ExtractLinks(body, markdown.Options{})
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse markdown links: %w", parseErr)
	}

	bodyStr := string(body)
	brokenLinks := make([]BrokenLink, 0)
	for _, link := range links {
		target := strings.TrimSpace(link.Destination)
		if target == "" {
			continue
		}

		if isHugoShortcodeLinkTarget(target) {
			continue
		}
		if isUIDAliasLinkTarget(target) {
			continue
		}

		// Skip external URLs and fragment-only links.
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			continue
		}
		if strings.HasPrefix(target, "#") {
			continue
		}
		// mailto: is not a local file.
		if strings.HasPrefix(target, "mailto:") {
			continue
		}

		lineNum := findLineNumberForTarget(bodyStr, target)

		switch link.Kind {
		case markdown.LinkKindImage:
			if isBrokenLink(sourceFile, target) {
				brokenLinks = append(brokenLinks, BrokenLink{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					Target:     target,
					LinkType:   LinkTypeImage,
				})
			}
		case markdown.LinkKindReferenceDefinition:
			if isBrokenLink(sourceFile, target) {
				brokenLinks = append(brokenLinks, BrokenLink{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					Target:     target,
					LinkType:   LinkTypeReference,
				})
			}
		case markdown.LinkKindInline, markdown.LinkKindAuto, markdown.LinkKindReference:
			if isBrokenLink(sourceFile, target) {
				brokenLinks = append(brokenLinks, BrokenLink{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					Target:     target,
					LinkType:   LinkTypeInline,
				})
			}
		default:
			// Unknown kinds are ignored for now.
		}
	}

	return brokenLinks, nil
}

func findLineNumberForTarget(body, target string) int {
	if body == "" || target == "" {
		return 1
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.Contains(line, target) {
			return i + 1
		}
	}
	return 1
}

// isBrokenLink checks if a link target points to a non-existent file.
func isBrokenLink(sourceFile, linkTarget string) bool {
	resolved, err := resolveRelativePath(sourceFile, linkTarget)
	if err != nil {
		return false
	}
	return !fileExists(resolved)
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
