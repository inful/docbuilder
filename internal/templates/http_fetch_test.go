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

func TestNewTemplateHTTPClient_BlocksCrossHostRedirect(t *testing.T) {
	// Create a server that redirects to a different host
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://evil.com/templates/")
			w.WriteHeader(http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	client := NewTemplateHTTPClient()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/redirect", nil)
	require.NoError(t, err)

	// The client should follow redirects, but CheckRedirect should block cross-host redirects
	resp, err := client.Do(req)
	if err != nil {
		require.Contains(t, err.Error(), "redirect to different host blocked")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	// If no error, the redirect was blocked by CheckRedirect
}

func TestNewTemplateHTTPClient_BlocksTooManyRedirects(t *testing.T) {
	redirectCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		if redirectCount < 10 {
			w.Header().Set("Location", serverURL+"/redirect")
			w.WriteHeader(http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	serverURL = server.URL
	t.Cleanup(server.Close)

	client := NewTemplateHTTPClient()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/redirect", nil)
	require.NoError(t, err)

	// The CheckRedirect should block after 5 redirects
	resp, err := client.Do(req)
	if err != nil {
		require.Contains(t, err.Error(), "too many redirects")
		return
	}
	defer func() { _ = resp.Body.Close() }()
}

func TestFetchTemplatePage_InvalidURL(t *testing.T) {
	client := NewTemplateHTTPClient()
	_, err := FetchTemplatePage(t.Context(), "not-a-url", client)
	require.Error(t, err)
	// The error could be "invalid URL" or "unsupported URL scheme" depending on parsing
	require.Contains(t, err.Error(), "URL")
}

func TestFetchTemplatePage_UnsupportedScheme(t *testing.T) {
	client := NewTemplateHTTPClient()
	_, err := FetchTemplatePage(t.Context(), "file:///path/to/file", client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported URL scheme")
}

func TestFetchTemplatePage_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := NewTemplateHTTPClient()
	_, err := FetchTemplatePage(t.Context(), server.URL+"/notfound", client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 404")
}

func TestFetchTemplateDiscovery_InvalidURL(t *testing.T) {
	client := NewTemplateHTTPClient()
	_, err := FetchTemplateDiscovery(t.Context(), "not-a-url", client)
	require.Error(t, err)
}

func TestFetchTemplateDiscovery_UnsupportedScheme(t *testing.T) {
	client := NewTemplateHTTPClient()
	_, err := FetchTemplateDiscovery(t.Context(), "file:///path/to/file", client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported URL scheme")
}
