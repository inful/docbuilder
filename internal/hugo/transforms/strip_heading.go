package transforms

import (
	"fmt"
	"regexp"
)

// stripFirstHeadingTransform removes the first H1 heading from markdown content
// to avoid duplication with Hugo's title rendering from frontmatter.
type stripFirstHeadingTransform struct{}

func (t stripFirstHeadingTransform) Name() string { return "strip_first_heading" }

func (t stripFirstHeadingTransform) Priority() int { return 50 } // Run early, before link rewrites

func (t stripFirstHeadingTransform) Transform(p PageAdapter) error {
	pg, ok := p.(*PageShim)
	if !ok {
		return fmt.Errorf("strip_first_heading: unexpected page adapter type")
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
