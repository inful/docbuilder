package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

// findLinksToFile finds all markdown links that reference the given target file.
// It searches from rootPath (typically the documentation root directory) to find
// all markdown files that might contain links to the target.
func (f *Fixer) findLinksToFile(targetPath, rootPath string) ([]LinkReference, error) {
	var links []LinkReference

	// Get absolute path of target for comparison
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// Ensure rootPath is a directory
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
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
			return fmt.Errorf("failed to scan %s: %w", path, err)
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
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	body := content
	lineOffset := 0
	fmRaw, fmBody, had, style, splitErr := frontmatter.Split(content)
	_ = style
	if splitErr == nil {
		body = fmBody
		if had {
			// frontmatter.Split removes:
			// - opening delimiter line
			// - fmRaw (which may span multiple lines)
			// - closing delimiter line
			// We need link line numbers to refer to the *original file* so that
			// applyLinkUpdates edits the correct line.
			lineOffset = 2 + strings.Count(string(fmRaw), "\n")
		}
	}

	bodyStr := string(body)

	links, parseErr := findLinksInBodyWithGoldmark(body, bodyStr, sourceFile, targetPath, lineOffset)
	if parseErr != nil {
		return nil, parseErr
	}

	return links, nil
}

func findLinksInBodyWithGoldmark(body []byte, bodyStr string, sourceFile, targetPath string, lineOffset int) ([]LinkReference, error) {
	parsedLinks, parseErr := markdown.ExtractLinks(body, markdown.Options{})
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse markdown links: %w", parseErr)
	}

	links := make([]LinkReference, 0)
	lines := strings.Split(bodyStr, "\n")
	skippable := computeSkippableLines(lines)
	searchStartLineByNeedle := make(map[string]int)

	for _, link := range parsedLinks {
		// Maintain parity with the current fixer: only inline links, images, and
		// reference definitions are discoverable for updates.
		var linkType LinkType
		switch link.Kind {
		case markdown.LinkKindInline:
			linkType = LinkTypeInline
		case markdown.LinkKindImage:
			linkType = LinkTypeImage
		case markdown.LinkKindReferenceDefinition:
			linkType = LinkTypeReference
		case markdown.LinkKindAuto, markdown.LinkKindReference:
			continue
		}

		dest := strings.TrimSpace(link.Destination)
		if dest == "" {
			continue
		}
		if isExternalURL(dest) {
			continue
		}
		if strings.HasPrefix(dest, "#") {
			continue
		}

		resolved, err := resolveRelativePath(sourceFile, dest)
		if err != nil {
			continue
		}
		if !pathsEqualCaseInsensitive(resolved, targetPath) {
			continue
		}

		needleKey := string(link.Kind) + "\x00" + dest
		lineInBody := findNextLineNumberForTargetInUnskippedLines(lines, skippable, dest, searchStartLineByNeedle[needleKey])
		searchStartLineByNeedle[needleKey] = lineInBody + 1
		lineNum := lineOffset + lineInBody

		fragment := ""
		targetNoFrag := dest
		if idx := strings.Index(dest, "#"); idx != -1 {
			fragment = dest[idx:]
			targetNoFrag = dest[:idx]
		}

		ref := LinkReference{
			SourceFile: sourceFile,
			LineNumber: lineNum,
			LinkType:   linkType,
			Target:     targetNoFrag,
			Fragment:   fragment,
			FullMatch:  "",
		}
		links = append(links, ref)
	}

	return links, nil
}

func computeSkippableLines(lines []string) []bool {
	skippable := make([]bool, len(lines))
	inCodeBlock := false
	activeFence := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock, activeFence = toggleFencedBlock(inCodeBlock, activeFence, "```")
			skippable[i] = true
			continue
		}
		if strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock, activeFence = toggleFencedBlock(inCodeBlock, activeFence, "~~~")
			skippable[i] = true
			continue
		}

		if inCodeBlock || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			skippable[i] = true
			continue
		}
	}

	return skippable
}

func toggleFencedBlock(inCodeBlock bool, activeFence string, fence string) (bool, string) {
	if !inCodeBlock {
		return true, fence
	}
	if activeFence == fence {
		return false, ""
	}
	return inCodeBlock, activeFence
}

func findNextLineNumberForTargetInUnskippedLines(lines []string, skippable []bool, target string, startLine int) int {
	if startLine < 1 {
		startLine = 1
	}
	if startLine > len(lines) {
		startLine = len(lines)
	}

	for i := startLine - 1; i < len(lines); i++ {
		if i >= 0 && i < len(skippable) && skippable[i] {
			continue
		}
		if strings.Contains(lines[i], target) {
			return i + 1
		}
	}

	return 1
}
