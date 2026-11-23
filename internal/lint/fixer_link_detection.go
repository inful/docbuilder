package lint

import (
	"fmt"
	"log/slog"
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

	// The root is a directory, or the file the fixer works on. The file
	// can be one that was renamed a moment ago - the fixer renames a
	// file and then looks for the links to it - so the directory around
	// it is the root when the root is not there any more.
	searchRoot := rootPath
	rootInfo, statErr := os.Stat(rootPath)
	if statErr != nil || !rootInfo.IsDir() {
		searchRoot = filepath.Dir(rootPath)
	}

	// The files of the tree are collected once: the fixer looks for the
	// links to every renamed file, and the walk was repeated for each of
	// them.
	files, err := f.markdownFilesOf(searchRoot)
	if err != nil {
		return nil, err
	}

	for _, path := range files {
		// Don't search the target file itself
		if path == targetPath {
			continue
		}

		// Find links in this file. A file that cannot be read does not
		// stop the scan: the links of the other files are still found.
		fileLinks, err := f.findLinksInFile(path, absTarget)
		if err != nil {
			slog.Warn("Failed to scan a file for links", "file", path, "error", err)
			continue
		}

		links = append(links, fileLinks...)
	}

	return links, nil
}

// markdownFilesOf returns the markdown files of a directory tree.
//
// The list is kept: the fixer looks for the links to every renamed
// file, and the walk of the tree was repeated for each of them.
func (f *Fixer) markdownFilesOf(root string) ([]string, error) {
	if f.scannedRoot == root && f.scannedFiles != nil {
		return f.scannedFiles, nil
	}

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !IsDocFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	f.scannedRoot = root
	f.scannedFiles = files
	return files, nil
}

// resolvedLink is one link from a markdown file, with the path it
// points at already resolved.
type resolvedLink struct {
	ref      docmodel.LinkRef
	resolved string // the path the link points at
	linkType LinkType
	target   string // the destination without the fragment
	fragment string
}

// parsedLinks returns the links of one markdown file.
//
// The result is cached. Reading and parsing a file is the slow part
// of looking for the links to a renamed file, and the fixer looks for
// them once per renamed file. The cache entry carries the size and
// the modification time of the file it was read from: an entry that no
// longer matches the file is read again, so a file that changed since
// the last read never yields stale links.
func (f *Fixer) parsedLinks(sourceFile string) ([]resolvedLink, error) {
	if entry, ok := f.linkCache[sourceFile]; ok && cachedLinksMatch(sourceFile, entry) {
		return entry.links, nil
	}

	doc, err := docmodel.ParseFile(sourceFile, docmodel.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	refs, err := doc.LinkRefs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown links: %w", err)
	}

	links := make([]resolvedLink, 0, len(refs))
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

		fragment := ""
		targetNoFrag := dest
		if idx := strings.Index(dest, "#"); idx != -1 {
			fragment = dest[idx:]
			targetNoFrag = dest[:idx]
		}

		links = append(links, resolvedLink{
			ref:      ref,
			resolved: resolved,
			linkType: linkType,
			target:   targetNoFrag,
			fragment: fragment,
		})
	}

	if f.linkCache == nil {
		f.linkCache = make(map[string]cachedLinks)
	}
	entry := cachedLinks{links: links}
	// The state of the file the links were read from. A file that
	// cannot be stat'd is recorded without one, so the cached links
	// stay usable when the file is renamed away.
	if info, statErr := os.Stat(sourceFile); statErr == nil {
		entry.size = info.Size()
		entry.modTime = info.ModTime()
	}
	f.linkCache[sourceFile] = entry
	return links, nil
}

// cachedLinksMatch reports whether the cached links still describe the
// file: it is unchanged, or it is gone because it was renamed away.
func cachedLinksMatch(sourceFile string, entry cachedLinks) bool {
	info, err := os.Stat(sourceFile)
	if err != nil {
		// The file is gone: the cached links are the ones we want.
		return true
	}
	return info.Size() == entry.size && info.ModTime().Equal(entry.modTime)
}

// findLinksInFile scans a single markdown file for links to the target.
func (f *Fixer) findLinksInFile(sourceFile, targetPath string) ([]LinkReference, error) {
	parsed, err := f.parsedLinks(sourceFile)
	if err != nil {
		return nil, err
	}

	links := make([]LinkReference, 0)
	for _, link := range parsed {
		if !pathsEqualCaseInsensitive(link.resolved, targetPath) {
			continue
		}

		links = append(links, LinkReference{
			SourceFile: sourceFile,
			LineNumber: link.ref.FileLine,
			LinkType:   link.linkType,
			Target:     link.target,
			Fragment:   link.fragment,
			FullMatch:  "",
		})
	}

	return links, nil
}
