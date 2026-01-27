package lint

import (
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
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
	doc, err := docmodel.ParseFile(sourceFile, docmodel.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	refs, err := doc.LinkRefs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown links: %w", err)
	}

	brokenLinks := make([]BrokenLink, 0)
	for _, ref := range refs {
		link := ref.Link
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
		// Bare email addresses (often from Markdown autolinks like <user@example.com>)
		// are not local files.
		if isBareEmailAddress(target) {
			continue
		}

		lineNum := ref.FileLine

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

func isBareEmailAddress(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// If it already looks like a scheme or a path, it's not a bare email.
	if strings.Contains(s, "://") || strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return false
	}
	// Basic shape: local@domain
	local, domain, ok := strings.Cut(s, "@")
	if !ok {
		return false
	}
	if local == "" || domain == "" {
		return false
	}
	// Avoid obvious non-emails.
	if strings.ContainsAny(s, " <>\t\n\r") {
		return false
	}
	// Require a dot in the domain to reduce false positives.
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
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
