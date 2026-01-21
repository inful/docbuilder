package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
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
	doc, err := docmodel.ParseFile(sourceFile, docmodel.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	refs, err := doc.LinkRefs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown links: %w", err)
	}

	links := make([]LinkReference, 0)
	for _, ref := range refs {
		link := ref.Link
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

		fragment := ""
		targetNoFrag := dest
		if idx := strings.Index(dest, "#"); idx != -1 {
			fragment = dest[idx:]
			targetNoFrag = dest[:idx]
		}

		links = append(links, LinkReference{
			SourceFile: sourceFile,
			LineNumber: ref.FileLine,
			LinkType:   linkType,
			Target:     targetNoFrag,
			Fragment:   fragment,
			FullMatch:  "",
		})
	}

	return links, nil
}
