package templates

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchTemplateDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/categories/templates/" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<a href="/templates/adr.template/">adr.template</a>`))
	}))
	t.Cleanup(server.Close)

	client := NewTemplateHTTPClient()
	templates, err := FetchTemplateDiscovery(t.Context(), server.URL, client)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, "adr", templates[0].Type)
	require.Equal(t, server.URL+"/templates/adr.template/", templates[0].URL)
}

func TestFetchTemplatePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/templates/adr.template/" {
			http.NotFound(w, r)
			return
		}
		page := `
			<html>
				<head>
					<meta property="docbuilder:template.type" content="adr">
					<meta property="docbuilder:template.name" content="ADR">
					<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
				</head>
				<body>
					<pre><code class="language-markdown"># body</code></pre>
				</body>
			</html>`
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(server.Close)

	client := NewTemplateHTTPClient()
	page, err := FetchTemplatePage(t.Context(), server.URL+"/templates/adr.template/", client)
	require.NoError(t, err)
	require.Equal(t, "adr", page.Meta.Type)
	require.Equal(t, "# body", strings.TrimSpace(page.Body))
}
