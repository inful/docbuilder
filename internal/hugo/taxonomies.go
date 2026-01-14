package hugo

import (
	"os"
	"path/filepath"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// copyTaxonomyLayouts creates custom taxonomy term layouts to avoid Relearn v9's
// children shortcode rendering issue where shortcode parameters appear as literal text.
// We override the term.html template to prevent Relearn from calling the children shortcode
// which causes its parameters to leak into the page output.
func (g *Generator) copyTaxonomyLayouts() error {
	// Create custom term.html template that properly integrates with Relearn's baseof.html
	// Relearn expects term.html to define "body" block, not "main"
	termTemplate := `{{- define "storeOutputFormat" }}
        {{- .Store.Set "relearnOutputFormat" "html" }}
{{- end }}
{{- define "body" }}
<article>
  <header class="headline">
  </header>

{{- $title := partial "title.gotmpl" (dict "page" .) }}
<h1 id="{{ $title | plainify | anchorize }}">{{ $title }}</h1>

{{- if .Pages -}}
<ul class="taxonomy-term-list">
{{- range .Pages -}}
<li>
  <h3><a href="{{ .RelPermalink }}">{{ .Title }}</a></h3>
  {{- with .Description -}}
  <p class="description">{{ . }}</p>
  {{- end -}}
</li>
{{- end -}}
</ul>
{{- else -}}
<p>No pages found with this {{ .Data.Singular }}.</p>
{{- end -}}

  <footer class="footline">
  </footer>
</article>
{{- end }}
{{- define "menu" }}
        {{- partial "menu.html" . }}
{{- end }}
`

	// Create term.html in multiple locations for compatibility
	locations := []string{
		filepath.Join(g.buildRoot(), "layouts", "tags"),
		filepath.Join(g.buildRoot(), "layouts", "categories"),
		filepath.Join(g.buildRoot(), "layouts", "taxonomy"),
		filepath.Join(g.buildRoot(), "layouts", "_default"),
	}

	for _, layoutsDir := range locations {
		if err := os.MkdirAll(layoutsDir, 0o750); err != nil {
			return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
				"failed to create layouts directory").
				WithContext("directory", layoutsDir).
				Build()
		}

		termPath := filepath.Join(layoutsDir, "term.html")
		// #nosec G306 -- layout files are public templates
		if err := os.WriteFile(termPath, []byte(termTemplate), 0o644); err != nil {
			return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
				"failed to write term.html template").
				WithContext("layouts_dir", layoutsDir).
				WithContext("term_path", termPath).
				Build()
		}
	}

	return nil
}
