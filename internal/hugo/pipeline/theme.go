package pipeline

import (
	_ "embed"
	"fmt"
	"path/filepath"
)

// DocsyVersion is the Docsy release the Docsy theme path builds
// against. It is the Hugo module version, so it carries the "v"
// prefix: the theme lives in the "theme" subdirectory of the Docsy
// repository, and Go tags such subdirectory modules "theme/vX.Y.Z".
//
// The vendored assets under assets/docsy must match this version;
// TestVendoredDocsyAssetsMatchPinnedVersion enforces that.
const DocsyVersion = "v0.18.1"

// DocsyModulePath is the Hugo module path of the Docsy theme.
const DocsyModulePath = "github.com/docsy/docsy/theme"

// The vendored Docsy assets. Docsy's stylesheet is compiled from SCSS
// that imports Bootstrap and Font Awesome from npm, and Hugo 0.170
// has no Sass support of its own. The compiled stylesheet, the
// Bootstrap JavaScript and the two web fonts it references are shipped
// in the repository instead; scripts/vendor-docsy-assets.sh rebuilds
// them.
//
//go:embed assets/docsy/docsy.css
var docsyStylesheet []byte

//go:embed assets/docsy/bootstrap.bundle.js
var docsyBootstrapJS []byte

//go:embed assets/docsy/fa-brands-400.woff2
var docsyFontBrands []byte

//go:embed assets/docsy/fa-solid-900.woff2
var docsyFontSolid []byte

//go:embed assets/docsy/VERSION
var docsyVendoredVersion string

//go:embed assets/docsy/head.html
var docsyHeadHTML []byte

//go:embed assets/docsy/lunr.min.js
var docsyLunrJS []byte

//go:embed assets/docsy-page-meta-links.html
var docsyPageMetaLinks []byte

// docsyHeadCSS is the project-level override of the theme's
// stylesheet partial. It links the vendored stylesheet instead of
// asking Hugo to compile the theme's SCSS.
const docsyHeadCSS = `{{- $css := resources.Get "css/docsy.css" | minify | fingerprint -}}
<link href="{{ $css.RelPermalink }}" rel="stylesheet" integrity="{{ $css.Data.Integrity }}" crossorigin="anonymous">
`

// docsyPageMetaLinksPath is where the page metadata override lands in
// the Hugo project. It shadows the theme's partial of the same name.
const docsyPageMetaLinksPath = "layouts/_partials/page-meta-links.html"

// docsyVendoredAssets returns the vendored Docsy files as static
// assets, keyed by their path relative to the Hugo project root.
//
// The stylesheet and the Bootstrap JavaScript live under assets/ so
// Hugo publishes them through its resource pipeline. The web fonts
// live under static/ because the stylesheet references them by a
// relative URL; the directory carries the Docsy version so a browser
// that cached the font does not keep it after a theme bump (the docs
// server gives fonts a one-year immutable cache header).
func docsyVendoredAssets() []*StaticAsset {
	return []*StaticAsset{
		{Path: "assets/css/docsy.css", Content: docsyStylesheet},
		{Path: "assets/vendor/bootstrap/dist/js/bootstrap.bundle.js", Content: docsyBootstrapJS},
		{Path: filepath.Join("assets", "vendor", "lunr", "lunr.min.js"), Content: docsyLunrJS},
		{Path: filepath.Join("static", "webfonts", DocsyVersion, "fa-brands-400.woff2"), Content: docsyFontBrands},
		{Path: filepath.Join("static", "webfonts", DocsyVersion, "fa-solid-900.woff2"), Content: docsyFontSolid},
	}
}

// indexContent renders the body of a generated index page.
//
// The theme renders the list of child pages itself, so the body is
// only the title and the description.
func indexContent(title, description string) string {
	return fmt.Sprintf("# %s\n\n%s\n\n", title, description)
}
