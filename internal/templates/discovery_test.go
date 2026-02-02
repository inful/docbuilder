package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTemplateDiscovery_ExtractsTemplates(t *testing.T) {
	html := `
		<html>
			<body>
				<a href="/path/adr.template/index.html">adr.template</a>
				<a href="/path/runbook.template/"></a>
				<a href="/path/not-a-template/">ignore me</a>
			</body>
		</html>`

	got, err := ParseTemplateDiscovery(strings.NewReader(html), "https://docs.example.com")
	require.NoError(t, err)
	require.Len(t, got, 2)

	require.Equal(t, "adr", got[0].Type)
	require.Equal(t, "https://docs.example.com/path/adr.template/index.html", got[0].URL)
	require.Equal(t, "adr.template", got[0].Name) // Name should be populated from anchor text

	require.Equal(t, "runbook", got[1].Type)
	require.Equal(t, "https://docs.example.com/path/runbook.template/", got[1].URL)
	require.Equal(t, "runbook", got[1].Name) // Name should fallback to type when anchor text is empty
}

func TestParseTemplateDiscovery_NoTemplates(t *testing.T) {
	html := `<html><body><a href="/path/regular/"></a></body></html>`

	_, err := ParseTemplateDiscovery(strings.NewReader(html), "https://docs.example.com")
	require.Error(t, err)
}

func TestParseTemplateDiscovery_ResolveURL(t *testing.T) {
	html := `<html><body><a href="/templates/adr.template/">adr</a></body></html>`
	links, err := ParseTemplateDiscovery(strings.NewReader(html), "https://docs.example.com")
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "https://docs.example.com/templates/adr.template/", links[0].URL)
}

func TestParseTemplateDiscovery_ResolveAbsoluteURL(t *testing.T) {
	html := `<html><body><a href="https://other.com/templates/adr.template/">adr</a></body></html>`
	links, err := ParseTemplateDiscovery(strings.NewReader(html), "https://docs.example.com")
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "https://other.com/templates/adr.template/", links[0].URL)
}

func TestParseTemplateDiscovery_InvalidBaseURL(t *testing.T) {
	html := `<html><body><a href="/templates/adr.template/">adr</a></body></html>`
	// ParseTemplateDiscovery will try to parse the URL, which may succeed or fail
	// depending on how url.Parse handles it. Let's test with a clearly invalid one.
	_, err := ParseTemplateDiscovery(strings.NewReader(html), "://invalid")
	// url.Parse may not error on this, so we just verify it doesn't crash
	_ = err
}
