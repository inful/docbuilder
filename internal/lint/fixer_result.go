package lint

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FixResult contains the results of a fix operation.
type FixResult struct {
	FilesRenamed  []RenameOperation
	LinksUpdated  []LinkUpdate
	BrokenLinks   []BrokenLink // Links to non-existent files
	ErrorsFixed   int
	WarningsFixed int
	Errors        []error
}

// RenameOperation represents a file rename operation.
type RenameOperation struct {
	OldPath string
	NewPath string
	Success bool
	Error   error
}

// LinkUpdate represents a link that was updated.
type LinkUpdate struct {
	SourceFile string
	LineNumber int
	OldTarget  string
	NewTarget  string
}

// BrokenLink represents a link to a non-existent file.
type BrokenLink struct {
	SourceFile string // File containing the broken link
	LineNumber int    // Line number of the link
	Target     string // Link target that doesn't exist
	LinkType   LinkType
}

// LinkType represents the type of markdown link.
type LinkType int

const (
	// LinkTypeInline represents inline markdown links: [text](url).
	LinkTypeInline LinkType = iota
	// LinkTypeReference represents reference-style links: [id]: url.
	LinkTypeReference
	// LinkTypeImage represents image links: ![alt](url).
	LinkTypeImage
)

// LinkReference represents a link found in a markdown file.
type LinkReference struct {
	SourceFile string   // File containing the link
	LineNumber int      // Line number of link
	LinkType   LinkType // Inline, Reference, or Image
	Target     string   // Link target (path)
	Fragment   string   // Anchor fragment (#section)
	FullMatch  string   // Complete original text for replacement
}

// HasErrors returns true if any errors occurred during fixing.
func (fr *FixResult) HasErrors() bool {
	return len(fr.Errors) > 0 || len(fr.BrokenLinks) > 0
}

// HasChanges returns true if there are any fixes to apply.
func (fr *FixResult) HasChanges() bool {
	return len(fr.FilesRenamed) > 0 || len(fr.LinksUpdated) > 0
}

// CountAffectedFiles returns the number of unique files that will be modified.
func (fr *FixResult) CountAffectedFiles() int {
	affected := make(map[string]bool)

	// Files being renamed
	for _, rename := range fr.FilesRenamed {
		affected[rename.OldPath] = true
	}

	// Files with link updates
	for _, update := range fr.LinksUpdated {
		affected[update.SourceFile] = true
	}

	return len(affected)
}

// Summary returns a human-readable summary of the fix operation.
func (fr *FixResult) Summary() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Files renamed: %d\n", len(fr.FilesRenamed)))
	b.WriteString(fmt.Sprintf("Errors fixed: %d\n", fr.ErrorsFixed))
	b.WriteString(fmt.Sprintf("Links updated: %d\n", len(fr.LinksUpdated)))
	b.WriteString(fmt.Sprintf("Warnings fixed: %d\n", fr.WarningsFixed))

	if len(fr.BrokenLinks) > 0 {
		b.WriteString(fmt.Sprintf("\nBroken links detected: %d\n", len(fr.BrokenLinks)))
		b.WriteString("These links reference non-existent files:\n")
		for _, broken := range fr.BrokenLinks {
			linkTypeStr := linkTypeInline
			switch broken.LinkType {
			case LinkTypeInline:
				linkTypeStr = linkTypeInline
			case LinkTypeImage:
				linkTypeStr = linkTypeImage
			case LinkTypeReference:
				linkTypeStr = linkTypeReference
			}
			b.WriteString(fmt.Sprintf("  • %s:%d: %s → %s (broken %s)\n",
				broken.SourceFile, broken.LineNumber, linkTypeStr, broken.Target, linkTypeStr))
		}
	}

	if len(fr.LinksUpdated) > 0 {
		b.WriteString("\nLink Updates:\n")
		for _, update := range fr.LinksUpdated {
			b.WriteString(fmt.Sprintf("  • %s:%d: %s → %s\n",
				update.SourceFile, update.LineNumber, update.OldTarget, update.NewTarget))
		}
	}

	if len(fr.Errors) > 0 {
		b.WriteString(fmt.Sprintf("\nErrors encountered: %d\n", len(fr.Errors)))
		for _, err := range fr.Errors {
			b.WriteString(fmt.Sprintf("  • %v\n", err))
		}
	}

	return b.String()
}

// PreviewChanges returns a detailed preview of what will be changed.
// This is shown to users before applying fixes for confirmation.
func (fr *FixResult) PreviewChanges() string {
	var b strings.Builder

	b.WriteString("The following changes will be made:\n\n")

	// File renames section
	if len(fr.FilesRenamed) > 0 {
		b.WriteString("FILE RENAMES:\n")
		for _, rename := range fr.FilesRenamed {
			oldName := filepath.Base(rename.OldPath)
			newName := filepath.Base(rename.NewPath)
			b.WriteString(fmt.Sprintf("  %s → %s\n", oldName, newName))
		}
		b.WriteString("\n")
	}

	// Links to update section
	if len(fr.LinksUpdated) > 0 {
		// Group by source file
		fileUpdates := make(map[string][]LinkUpdate)
		for _, update := range fr.LinksUpdated {
			filename := filepath.Base(update.SourceFile)
			fileUpdates[filename] = append(fileUpdates[filename], update)
		}

		b.WriteString("LINKS TO UPDATE:\n")
		for filename, updates := range fileUpdates {
			b.WriteString(fmt.Sprintf("  • %s (%d link%s)\n", filename, len(updates), pluralize(len(updates))))
		}
		b.WriteString("\n")
	}

	// Statistics
	b.WriteString("SUMMARY:\n")
	b.WriteString(fmt.Sprintf("  • %d file%s will be renamed\n", len(fr.FilesRenamed), pluralize(len(fr.FilesRenamed))))
	b.WriteString(fmt.Sprintf("  • %d link%s will be updated\n", len(fr.LinksUpdated), pluralize(len(fr.LinksUpdated))))
	if len(fr.BrokenLinks) > 0 {
		b.WriteString(fmt.Sprintf("  • %d broken link%s detected\n", len(fr.BrokenLinks), pluralize(len(fr.BrokenLinks))))
	}

	return b.String()
}

// DetailedPreview returns a line-by-line preview of what will change.
// This is shown in dry-run mode to see exact before/after states.
func (fr *FixResult) DetailedPreview() string {
	var b strings.Builder

	b.WriteString("DETAILED CHANGES PREVIEW\n")
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	// File renames with full paths
	if len(fr.FilesRenamed) > 0 {
		b.WriteString("[File Renames]\n")
		for i, rename := range fr.FilesRenamed {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, rename.OldPath))
			b.WriteString(fmt.Sprintf("   → %s\n\n", rename.NewPath))
		}
	}

	// Link updates with before/after
	if len(fr.LinksUpdated) > 0 {
		b.WriteString("[Link Updates]\n")
		for i, update := range fr.LinksUpdated {
			b.WriteString(fmt.Sprintf("%d. %s:%d\n", i+1, update.SourceFile, update.LineNumber))
			b.WriteString(fmt.Sprintf("   Before: %s\n", update.OldTarget))
			b.WriteString(fmt.Sprintf("   After:  %s\n\n", update.NewTarget))
		}
	}

	// Broken links section
	if len(fr.BrokenLinks) > 0 {
		b.WriteString("[Broken Links Detected]\n")
		for i, broken := range fr.BrokenLinks {
			linkTypeStr := linkTypeInline
			switch broken.LinkType {
			case LinkTypeInline:
				linkTypeStr = linkTypeInline
			case LinkTypeImage:
				linkTypeStr = linkTypeImage
			case LinkTypeReference:
				linkTypeStr = linkTypeReference
			}
			b.WriteString(fmt.Sprintf("%d. %s:%d (%s)\n", i+1, broken.SourceFile, broken.LineNumber, linkTypeStr))
			b.WriteString(fmt.Sprintf("   Target: %s (file not found)\n\n", broken.Target))
		}
	}

	b.WriteString(strings.Repeat("=", 60) + "\n")
	b.WriteString(fmt.Sprintf("Total: %d file%s, %d link%s\n",
		len(fr.FilesRenamed), pluralize(len(fr.FilesRenamed)),
		len(fr.LinksUpdated), pluralize(len(fr.LinksUpdated))))

	return b.String()
}
