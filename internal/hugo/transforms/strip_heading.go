package transforms

import (
	"fmt"
	"regexp"
	"strings"
)

// stripFirstHeadingTransform removes the first H1 heading from markdown content
// to avoid duplication with Hugo's title rendering from frontmatter.
// 
// EXCEPTION: index.md and README.md files become Hugo section indexes (_index.md)
// and need their H1 heading as the section title, so we preserve it for those files.
type stripFirstHeadingTransform struct{}

func (t stripFirstHeadingTransform) Name() string { return "strip_first_heading" }

func (t stripFirstHeadingTransform) Stage() TransformStage {
	return StageTransform
}

func (t stripFirstHeadingTransform) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"relative_link_rewriter"},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: false,
		ModifiesContent:             true,
		ModifiesFrontMatter:         false,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        false,
	}
}

func (t stripFirstHeadingTransform) Transform(p PageAdapter) error {
	pg, ok := p.(*PageShim)
	if !ok {
		return fmt.Errorf("strip_first_heading: unexpected page adapter type")
	}

	// Skip stripping for index.md and README.md files - they become Hugo section indexes
	// (_index.md) and themes like Relearn require the H1 heading as the section title
	fileName := strings.ToLower(pg.Doc.Name)
	if fileName == "index" || fileName == "readme" {
		return nil
	}

	// Pattern matches:
	// - Optional leading whitespace/newlines
	// - A single # followed by space and heading text
	// - The rest of the line (heading content)
	// - Optional trailing newlines after the heading
	pattern := regexp.MustCompile(`(?m)^\s*#\s+[^\n]+\n*`)

	content := pg.Content

	// Only strip the first occurrence
	loc := pattern.FindStringIndex(content)
	if loc != nil {
		pg.Content = content[:loc[0]] + content[loc[1]:]
	}

	return nil
}

func init() {
	Register(stripFirstHeadingTransform{})
}
