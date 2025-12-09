package transforms

import (
	"fmt"
	"regexp"
	"strings"
)

// ShortcodeEscaper escapes Hugo shortcodes within code blocks to prevent Hugo from processing them.
// It converts {{< ... >}} to {{</* ... */>}} and {{% ... %}} to {{%/* ... */%}} within fenced code blocks.
type ShortcodeEscaper struct{}

func (s ShortcodeEscaper) Name() string { return "shortcode_escaper" }

func (s ShortcodeEscaper) Stage() TransformStage {
	return StageFinalize
}

func (s ShortcodeEscaper) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"strip_first_heading"},
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

var (
	// Match fenced code blocks (```...``` or ~~~...~~~)
	codeBlockPattern = regexp.MustCompile("(?s)(```[^`]*```|~~~[^~]*~~~)")

	// Match Hugo shortcodes: {{< ... >}} or {{% ... %}}
	angleShortcode   = regexp.MustCompile(`\{\{<\s*([^>]+?)\s*>\}\}`)
	percentShortcode = regexp.MustCompile(`\{\{%\s*([^%]+?)\s*%\}\}`)
)

func (s ShortcodeEscaper) Transform(p PageAdapter) error {
	pg, ok := p.(*PageShim)
	if !ok {
		return fmt.Errorf("shortcode_escaper: unexpected page adapter type")
	}

	// Skip if not markdown
	if !strings.HasSuffix(pg.FilePath, ".md") && !strings.HasSuffix(pg.FilePath, ".markdown") {
		return nil
	}

	// Process each code block
	pg.Content = codeBlockPattern.ReplaceAllStringFunc(pg.Content, func(block string) string {
		// Escape angle bracket shortcodes: {{< foo >}} → {{</* foo */>}}
		block = angleShortcode.ReplaceAllString(block, `{{</* $1 */>}}`)

		// Escape percent shortcodes: {{% bar %}} → {{%/* bar */%}}
		block = percentShortcode.ReplaceAllString(block, `{{%/* $1 */%}}`)

		return block
	})

	return nil
}

func init() {
	Register(ShortcodeEscaper{})
}
