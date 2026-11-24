package pipeline

// katexPartial is the project's override of the theme's KaTeX partial.
//
// The theme fetches the stylesheet and the fonts from a CDN with
// resources.GetRemote, and the build fails when the network does not
// answer: the operator builds the site without a network, and the build
// died with "Could not retrieve KaTeX css file from CDN" (issue #129).
//
// The site serves its own copy: the stylesheet and the fonts are
// published through the resource pipeline of Hugo, so they are published
// when a page with math pulls the stylesheet in - and not in every
// build.
const katexPartial = `{{- $css := resources.Get "katex/katex.min.css" -}}
{{- with $css -}}
<link rel="stylesheet" href="{{- .RelPermalink -}}" integrity="{{- (. | fingerprint "sha512").Data.Integrity -}}" crossorigin="anonymous">
{{- end -}}
{{- range (resources.Match "katex/fonts/*.woff2") }}
{{- $_ := .RelPermalink }}
{{- end -}}
`

// mermaidPartial is the project's override of the theme's mermaid partial.
//
// The theme fetches the script from a CDN to check that the version in
// the configuration exists, and the build fails when the network does
// not answer (issue #129). The browser fetches the script anyway, so
// the check is dropped and the version is still pinned by the
// configuration.
const mermaidPartial = `{{- $version := .Plugin.version -}}
{{- if not $version -}}
  {{- errorf "Mermaid's effective version pin is empty. For configuration, see https://www.docsy.dev/docs/content/plugins/#adjust-a-plugin-per-page." -}}
{{- end -}}
{{- $options := dict -}}
{{- with .Plugin.options -}}
  {{- if reflect.IsMap . -}}
    {{- $options = . -}}
  {{- else -}}
    {{- errorf "params.docsy.plugins.mermaid.options reached the companion undecoded (%T): the mermaid shim decodes it, and a shim override must too." . -}}
  {{- end -}}
{{- end -}}
{{- $cdnurl := printf "https://cdn.jsdelivr.net/npm/mermaid@%s/dist/mermaid.esm.min.mjs" $version -}}
<script type="application/json" data-docsy-plugin="mermaid">{{ dict "url" $cdnurl "options" $options | jsonify | safeJS }}</script>
{{- "\n" -}}
`
