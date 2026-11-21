package pipeline

import (
	"bytes"
	_ "embed"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Embedded View Transitions API assets

//go:embed assets/view-transitions.css
var viewTransitionsCSS []byte

//go:embed assets/view-transitions-head.html
var viewTransitionsHeadPartial []byte

//go:embed assets/template-metadata-head.html
var templateMetadataHeadPartial []byte

// StaticAsset represents a static file to be copied to the Hugo site.
// Unlike Document, StaticAsset doesn't go through the transform pipeline.
type StaticAsset struct {
	Path    string // Relative path from Hugo root (e.g., "assets/css/view-transitions.css")
	Content []byte // Raw file content
}

// StaticAssetGenerator creates static assets based on configuration.
// Returns a list of assets to be copied to the Hugo site root.
type StaticAssetGenerator func(ctx *GenerationContext) ([]*StaticAsset, error)

// generateCustomHeaderAssets builds the project's layout overrides for
// the configured theme. The overrides carry three pieces, in this
// order:
//
//  1. A <style> block with the resolved hugo.custom_css (or the
//     built-in default that tones down Relearn's loud sidebar
//     section titles).
//  2. When EnablePageTransitions is true, the View Transitions
//     API <link> and the accompanying view-transitions.css asset.
//  3. The template-metadata <meta> tags that template discovery
//     depends on (always present).
//
// Relearn takes them in layouts/partials/custom-header.html, the
// override point Relearn documents for adding custom CSS. Docsy takes
// them in layouts/_partials/hooks/head-end.html, the hook Docsy
// includes in every page head. Docsy additionally gets a
// head-css.html override that links the vendored stylesheet, and the
// vendored stylesheet, Bootstrap JavaScript and web fonts themselves.
//
// When hugo.custom_css is unset, the built-in default is written.
// When set to a non-empty value, that value replaces the default
// verbatim. A whitespace-only value suppresses the <style> block
// entirely so operators who want the unmodified theme styling can opt
// out. Docsy has no built-in default: the Relearn override it would
// replace does not exist there.
func generateCustomHeaderAssets(ctx *GenerationContext) ([]*StaticAsset, error) {
	transitionsEnabled := ctx != nil && ctx.Config != nil && ctx.Config.Hugo.EnablePageTransitions
	docsy := IsDocsy(configOf(ctx))

	// Resolve the custom CSS: empty -> default, otherwise the
	// configured value. A whitespace-only value suppresses the
	// <style> block.
	css := ""
	if ctx != nil && ctx.Config != nil {
		css = ctx.Config.Hugo.CustomCSS
	}
	if css == "" && !docsy {
		css = defaultCustomCSS
	}
	includeCSS := strings.TrimSpace(css) != ""

	assets := make([]*StaticAsset, 0, 2)

	// Assemble the partial: optional <style>, optional view-
	// transitions <link>, template-metadata <meta>s.
	var blocks [][]byte
	if includeCSS {
		blocks = append(blocks, []byte("<style>\n"+css+"\n</style>"))
	}
	if transitionsEnabled {
		blocks = append(blocks, viewTransitionsHeadPartial)
		assets = append(assets, &StaticAsset{
			// The stylesheet lives in assets/ so the partial can publish
			// it through Hugo's resource pipeline: the docs server serves
			// .css with a one-year immutable cache header, which is only
			// safe for a content-addressed URL.
			Path:    "assets/css/view-transitions.css",
			Content: viewTransitionsCSS,
		})
	}
	blocks = append(blocks, templateMetadataHeadPartial)
	content := bytes.Join(blocks, []byte("\n"))

	if docsy {
		assets = append(assets,
			&StaticAsset{Path: "layouts/_partials/hooks/head-end.html", Content: content},
			&StaticAsset{Path: "layouts/_partials/head-css.html", Content: []byte(docsyHeadCSS)},
		)
		assets = append(assets, docsyVendoredAssets()...)
		return assets, nil
	}

	assets = append(assets, &StaticAsset{
		Path:    "layouts/partials/custom-header.html",
		Content: content,
	})

	return assets, nil
}

// configOf returns the build configuration of the generation context,
// or nil when there is none.
func configOf(ctx *GenerationContext) *config.Config {
	if ctx == nil {
		return nil
	}
	return ctx.Config
}
