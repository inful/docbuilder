package transforms

import (
	"regexp"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// extractIndexTitleTransform extracts the H1 heading from index.md and README.md files
// and adds it as the title in front matter. This is necessary because themes like Relearn
// require a title in the front matter for section indexes, but we preserve the H1 heading
// in the content (unlike regular pages where we strip it).
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

	// Extract H1 heading from content
	// Pattern matches: optional whitespace, single #, space, heading text
	pattern := regexp.MustCompile(`(?m)^\s*#\s+([^\n]+)`)
	matches := pattern.FindStringSubmatch(pg.Content)

	if len(matches) > 1 {
		title := strings.TrimSpace(matches[1])
		if title != "" {
			// Add title as a front matter patch
			patch := fmcore.FrontMatterPatch{
				Source:   "extract_index_title",
				Mode:     fmcore.MergeDeep,
				Priority: 55, // After builder_v2 (50) but before merge
				Data: map[string]any{
					"title": title,
				},
			}
			pg.AddPatch(patch)
		}
	}

	return nil
}

func init() {
	Register(extractIndexTitleTransform{})
}
