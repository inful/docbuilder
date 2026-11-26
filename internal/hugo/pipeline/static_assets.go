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

//go:embed assets/search/offline-search.js
var serverSideSearchJS []byte

//go:embed assets/permalink-copy.js
var permalinkCopyJS []byte

// permalinkCopyScriptPartial loads the script of the copy buttons of
// the permalink. The script is published through the resource pipeline
// of Hugo, so the browser gets it under a content-addressed URL.
const permalinkCopyScriptPartial = `{{- $js := resources.Get "js/permalink-copy.js" | fingerprint -}}
<script defer
  src="{{ $js.RelPermalink }}"
  integrity="{{ $js.Data.Integrity }}"
  crossorigin="anonymous"></script>
`

// emptySearchIndex is the search index of a site whose search runs on
// the server. The theme builds the index of every page into this file
// and the browser downloads it; with the server-side search the file
// stays empty, and the browser asks the server instead.
var emptySearchIndex = []byte("[]")

// StaticAsset represents a static file to be copied to the Hugo site.
// Unlike Document, StaticAsset doesn't go through the transform pipeline.
type StaticAsset struct {
	Path    string // Relative path from Hugo root (e.g., "assets/css/view-transitions.css")
	Content []byte // Raw file content
}

// StaticAssetGenerator creates static assets based on configuration.
// Returns a list of assets to be copied to the Hugo site root.
type StaticAssetGenerator func(ctx *GenerationContext) ([]*StaticAsset, error)

// generateThemeAssets builds the project's layout overrides for the
// theme. The overrides carry four pieces, in this order:
//
//  1. A <style> block with the resolved hugo.custom_css.
//  2. When EnablePageTransitions is true, the View Transitions
//     API <link> and the accompanying view-transitions.css asset.
//  3. The template-metadata <meta> tags that template discovery
//     depends on (always present).
//  4. The script of the copy buttons of the permalink (always
//     present).
//
// The theme takes them in layouts/_partials/hooks/head-end.html, the
// hook it includes in every page head. It also gets a head-css.html
// override that links the vendored stylesheet, and the vendored
// stylesheet, Bootstrap JavaScript and web fonts themselves.
//
// A whitespace-only hugo.custom_css suppresses the <style> block
// entirely, so operators who want the unmodified theme styling can
// opt out.
func generateThemeAssets(ctx *GenerationContext) ([]*StaticAsset, error) {
	transitionsEnabled := ctx != nil && ctx.Config != nil && ctx.Config.Hugo.EnablePageTransitions

	// The custom CSS: a whitespace-only value suppresses the block.
	css := ""
	if ctx != nil && ctx.Config != nil {
		css = ctx.Config.Hugo.CustomCSS
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
	blocks = append(blocks, templateMetadataHeadPartial,
		// The copy buttons of the permalink need their script on every
		// page.
		[]byte(permalinkCopyScriptPartial))
	content := bytes.Join(blocks, []byte("\n"))

	assets = append(assets,
		&StaticAsset{Path: "layouts/_partials/hooks/head-end.html", Content: content},
		&StaticAsset{Path: "layouts/_partials/head-css.html", Content: []byte(docsyHeadCSS)},
		&StaticAsset{Path: docsyPageMetaLinksPath, Content: docsyPageMetaLinks},
		// The script of the copy buttons of the permalink. It lives in
		// assets/ so the head hook can publish it through the resource
		// pipeline of Hugo.
		&StaticAsset{Path: "assets/js/permalink-copy.js", Content: permalinkCopyJS},
		// The children shortcode of the previous theme. The documentation
		// of the repositories uses it, and a shortcode without a template
		// fails the build of the whole site.
		&StaticAsset{Path: docsyChildrenShortcodePath, Content: []byte(docsyChildrenShortcode)},
		// The theme's own head partial, with the offline-search script
		// pointed at the vendored copy instead of the unpkg CDN.
		&StaticAsset{Path: "layouts/_partials/head.html", Content: docsyHeadHTML},
	)
	assets = append(assets, docsyVendoredAssets()...)

	// The theme fetches KaTeX from a CDN and mermaid from a CDN, and
	// the build fails when the network does not answer (issue #129). The
	// site serves its own copy of KaTeX, and the mermaid partial of the
	// theme is replaced by one that does not fetch anything.
	katexAssets, err := katexVendoredAssets()
	if err != nil {
		return nil, err
	}
	assets = append(assets, katexAssets...)
	assets = append(assets,
		&StaticAsset{Path: "layouts/_partials/scripts/katex.html", Content: []byte(katexPartial)},
		&StaticAsset{Path: "layouts/_partials/scripts/plugins/mermaid.html", Content: []byte(mermaidPartial)},
	)

	if serverSideSearch(ctx) {
		// The search runs on the server: the browser asks the daemon
		// instead of downloading an index of the whole site. The theme's
		// search script and its index are replaced: the script asks the
		// server, and the index stays empty because nobody reads it.
		assets = append(assets,
			&StaticAsset{Path: "assets/js/offline-search.js", Content: serverSideSearchJS},
			&StaticAsset{Path: "assets/json/offline-search-index.json", Content: emptySearchIndex},
		)
	}

	return assets, nil
}

// serverSideSearch reports whether the search of the site runs on the
// server.
func serverSideSearch(ctx *GenerationContext) bool {
	if ctx == nil || ctx.Config == nil || ctx.Config.Search == nil {
		return false
	}
	return ctx.Config.Search.Provider == config.SearchProviderServer
}
