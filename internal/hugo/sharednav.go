package hugo

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// relearnVersion is the Relearn release DocBuilder builds against. It is
// written into the generated hugo.yaml as the Hugo module version.
//
// The version is the release tag without a "v" prefix: Relearn tags its
// releases that way, and a "v" prefix makes the Go module resolution fail
// with "unknown revision". Hugo maps the tag onto the pseudo-version of the
// tagged commit, so the pin is honored.
//
// The vendored navigation template under assets/relearn must match this
// version; TestVendoredRelearnNavMatchesPinnedVersion enforces that.
const relearnVersion = "9.0.3"

//go:embed assets/relearn/menu.html
var relearnNavTemplate []byte

//go:embed assets/nav-shared.js
var navSharedScript []byte

// SharedNavOutputFormat is the name of the Hugo output format that carries
// the shared navigation fragment.
const SharedNavOutputFormat = "sidebar"

// sharedNavPlaceholder replaces the theme's sidebar partial. The page ships
// an empty sidebar plus the script that fills it in; the navigation itself
// lives in the shared fragment.
//
// The script is published through Hugo's resource pipeline so its URL is
// content-addressed. The docs server serves .js with a one-year immutable
// cache header, which is only safe for content-hashed URLs - a stable URL
// would keep serving a stale script after DocBuilder is upgraded.
//
// The script tag sits next to the sidebar, not inside it: the fragment
// replaces the sidebar's contents, which would take the tag with it.
const sharedNavPlaceholder = `<aside id="R-sidebar" class="default-animation">
  <div id="R-sidebar-menu" data-nav-src="{{ with site.Home.OutputFormats.Get "sidebar" }}{{ .RelPermalink }}{{ end }}"></div>
</aside>
{{- with resources.Get "js/nav-shared.js" }}
<script src="{{ (. | fingerprint "sha256").RelPermalink }}" defer></script>
{{- end }}
`

// sharedNavFragmentLayout renders the shared fragment: the navigation and
// nothing else, so the fragment is a bare piece of markup the browser can
// graft into a page.
const sharedNavFragmentLayout = `{{- partial "nav-full.html" . -}}
`

// SharedNavFiles returns the files to write into the Hugo project root for
// the shared navigation mode, keyed by their path relative to that root.
//
// The theme's own layouts/partials/menu.html is shadowed by a placeholder;
// the vendored copy is written alongside it under a different name so the
// fragment can still render the real navigation.
func SharedNavFiles() map[string]string {
	return map[string]string{
		filepath.Join("layouts", "partials", "menu.html"):         sharedNavPlaceholder,
		filepath.Join("layouts", "partials", "nav-full.html"):     string(relearnNavTemplate),
		filepath.Join("layouts", "_default", "home.sidebar.html"): sharedNavFragmentLayout,
		// The script lives in assets/ so the placeholder can publish it
		// through Hugo's resource pipeline with a content-addressed URL.
		filepath.Join("assets", "js", "nav-shared.js"): string(navSharedScript),
	}
}

// SharedNavOutputFormats returns the Hugo root-config block that defines the
// output format used for the shared navigation fragment.
func SharedNavOutputFormats() map[string]any {
	return map[string]any{
		SharedNavOutputFormat: map[string]any{
			"name":          SharedNavOutputFormat,
			"baseName":      SharedNavOutputFormat,
			"isHTML":        true,
			"mediaType":     "text/html",
			"permalinkable": false,
			"noUgly":        true,
		},
	}
}

// WriteSharedNavFiles writes the shared-navigation overrides into the Hugo
// project root, shadowing the theme's sidebar partial.
func (g *Generator) WriteSharedNavFiles() error {
	root := g.BuildRoot()
	for rel, content := range SharedNavFiles() {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("shared sidebar: create %s: %w", filepath.Dir(path), err)
		}
		// #nosec G306 -- the path is inside the build root
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("shared sidebar: write %s: %w", path, err)
		}
	}
	slog.Info("Wrote shared navigation files", "root", root, "files", len(SharedNavFiles()))
	return nil
}
