package transforms

import (
	"regexp"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// extractIndexTitleTransform sets the title for index.md and README.md files.
// For index files within a section (folder), it uses the section/folder name as the title.
// For root-level indexes, it extracts the H1 heading from the content.
// This ensures consistent navigation titles based on folder structure while supporting
// themes like Relearn that require titles in front matter.
type extractIndexTitleTransform struct{}

func (t extractIndexTitleTransform) Name() string { return "extract_index_title" }

func (t extractIndexTitleTransform) Stage() TransformStage {
	return StageBuild
}

func (t extractIndexTitleTransform) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"front_matter_builder_v2"},
		MustRunBefore:               []string{"front_matter_merge"},
		RequiresOriginalFrontMatter: true,
		ModifiesContent:             false,
		ModifiesFrontMatter:         true,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        false,
	}
}

func (t extractIndexTitleTransform) Transform(p PageAdapter) error {
	pg, ok := p.(*PageShim)
	if !ok {
		return nil
	}

	// Only process index.md and README.md files
	fileName := strings.ToLower(pg.Doc.Name)
	if fileName != "index" && fileName != "readme" {
		return nil
	}

	// Check if title already exists in original front matter
	if pg.OriginalFrontMatter != nil {
		if title, hasTitle := pg.OriginalFrontMatter["title"]; hasTitle && title != nil && title != "" {
			// Title already exists, no need to extract from heading
			return nil
		}
	}

	var title string

	// For index files with a section, use the section name as the title
	// This ensures folder names become navigation titles (e.g., "vcfretriever" folder → "Vcfretriever" title)
	if pg.Doc.Section != "" {
		title = titleCase(pg.Doc.Section)
	} else {
		// For root-level indexes, check if content starts with H1 or has text before it
		// Pattern matches: optional whitespace, single #, space, heading text
		pattern := regexp.MustCompile(`(?m)^\s*#\s+([^\n]+)`)
		loc := pattern.FindStringIndex(pg.Content)
		
		if loc != nil {
			// Found H1 - check if there's any non-whitespace text before it
			textBeforeH1 := strings.TrimSpace(pg.Content[:loc[0]])
			
			if textBeforeH1 == "" {
				// No text before H1 - extract H1 as title
				// (the H1 will be stripped by strip_first_heading)
				matches := pattern.FindStringSubmatch(pg.Content)
				if len(matches) > 1 {
					title = strings.TrimSpace(matches[1])
				}
			}
			// else: text before H1 exists - don't extract title (use default from filename)
		}
	}

	if title != "" {
		// Add title as a front matter patch using MergeReplace to override protected key
		patch := fmcore.FrontMatterPatch{
			Source:   "extract_index_title",
			Mode:     fmcore.MergeReplace, // Use Replace mode to override the protected "title" key
			Priority: 55,                  // After builder_v2 (50) but before merge
			Data: map[string]any{
				"title": title,
			},
		}
		pg.AddPatch(patch)
	}

	return nil
}

// titleCase converts a string to title case by replacing dashes and underscores with spaces
// and capitalizing the first letter of each word.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	parts := strings.Fields(s)
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

func init() {
	Register(extractIndexTitleTransform{})
}
