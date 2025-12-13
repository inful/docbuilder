package hugo

import (
	"os"
	"path/filepath"
)

// generateBasicLayouts creates basic Hugo layout templates
func (g *Generator) generateBasicLayouts() error {
	layouts := map[string]string{
		"layouts/_default/baseof.html":      baseofTemplate,
		"layouts/_default/single.html":      singleTemplate,
		"layouts/_default/list.html":        listTemplate,
		"layouts/partials/head.html":        headTemplate,
		"layouts/partials/header.html":      headerTemplate,
		"layouts/partials/footer.html":      footerTemplate,
		"layouts/partials/transitions.html": transitionsPartial,
		"layouts/index.html":                indexTemplate,
	}

	for path, content := range layouts {
		fullPath := filepath.Join(g.buildRoot(), path)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
			return err
		}

		// #nosec G306 -- layout templates are public assets
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// HTML Templates
const baseofTemplate = `<!DOCTYPE html>
<html lang="{{ .Site.LanguageCode | default "en" }}">
<head>
  {{ partial "head.html" . }}
</head>
<body>
  {{ partial "header.html" . }}
  <main>
    {{ block "main" . }}{{ end }}
  </main>
  {{ partial "footer.html" . }}
</body>
</html>`

const singleTemplate = `{{ define "main" }}
<article>
  <header>
    <h1>{{ .Title }}</h1>
    {{ if .Params.repository }}
    <p><strong>Repository:</strong> {{ .Params.repository }}</p>
    {{ end }}
    {{ if .Params.section }}
    <p><strong>Section:</strong> {{ .Params.section }}</p>
    {{ end }}
  </header>
  <div class="content">
    {{ .Content }}
  </div>
</article>
{{ end }}`

const listTemplate = `{{ define "main" }}
<section>
  <header>
    <h1>{{ .Title }}</h1>
    {{ if .Content }}
    <div class="description">
      {{ .Content }}
    </div>
    {{ end }}
  </header>
  
  {{ if .Pages }}
  <div class="page-list">
    {{ range .Pages }}
    <article>
      <h2><a href="{{ .Permalink }}">{{ .Title }}</a></h2>
      {{ if .Summary }}
      <p>{{ .Summary }}</p>
      {{ end }}
      <div class="meta">
        {{ if .Params.repository }}
        <span>Repository: {{ .Params.repository }}</span>
        {{ end }}
        {{ if .Params.section }}
        <span>Section: {{ .Params.section }}</span>
        {{ end }}
      </div>
    </article>
    {{ end }}
  </div>
  {{ end }}
</section>
{{ end }}`

const headTemplate = `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{ if .Title }}{{ .Title }} - {{ end }}{{ .Site.Title }}</title>
{{ if .Description }}
<meta name="description" content="{{ .Description }}">
{{ else if .Site.Params.description }}
<meta name="description" content="{{ .Site.Params.description }}">
{{ end }}
{{ partial "transitions.html" . }}
<style>
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  line-height: 1.6;
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
  color: #333;
}
nav {
  border-bottom: 1px solid #eee;
  margin-bottom: 2rem;
  padding-bottom: 1rem;
}
nav a {
  margin-right: 1rem;
  text-decoration: none;
  color: #0066cc;
}
nav a:hover {
  text-decoration: underline;
}
.content {
  margin: 2rem 0;
}
.page-list article {
  border-bottom: 1px solid #eee;
  padding: 1rem 0;
}
.meta {
  color: #666;
  font-size: 0.9em;
}
.meta span {
  margin-right: 1rem;
}
pre {
  background: #f5f5f5;
  padding: 1rem;
  overflow-x: auto;
}
code {
  background: #f5f5f5;
  padding: 0.2rem 0.4rem;
  border-radius: 3px;
}
pre code {
  background: none;
  padding: 0;
}
</style>`

const headerTemplate = `<header>
  <nav>
    <a href="{{ "/" | relURL }}">{{ .Site.Title }}</a>
    {{ range .Site.Menus.main }}
    <a href="{{ .URL }}">{{ .Name }}</a>
    {{ end }}
  </nav>
</header>`

const footerTemplate = `<footer>
  <hr>
  <p>Generated with docbuilder on {{ .Site.Params.build_date }}</p>
</footer>`

const indexTemplate = `{{ define "main" }}
<section>
  <h1>{{ .Site.Title }}</h1>
  {{ if .Site.Params.description }}
  <p>{{ .Site.Params.description }}</p>
  {{ end }}
  
  {{ .Content }}
  
  {{ if .Pages }}
  <h2>Documentation Sections</h2>
  <div class="page-list">
    {{ range .Pages }}
    <article>
      <h3><a href="{{ .Permalink }}">{{ .Title }}</a></h3>
      {{ if .Summary }}
      <p>{{ .Summary }}</p>
      {{ end }}
    </article>
    {{ end }}
  </div>
  {{ end }}
</section>
{{ end }}`

const transitionsPartial = `{{ if .Site.Params.enable_transitions -}}
{{- $duration := .Site.Params.transition_duration | default "300ms" -}}
<link rel="stylesheet" href="{{ "view-transitions.css" | relURL }}">
<script src="{{ "view-transitions.js" | relURL }}" defer></script>
<style>
:root {
  --view-transition-duration: {{ $duration }};
}
</style>
{{- end }}
`
