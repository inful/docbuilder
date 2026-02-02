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

	got, err := ParseTemplateDiscovery("https://docs.example.com", strings.NewReader(html))
	require.NoError(t, err)
	require.Len(t, got, 2)

	require.Equal(t, "adr", got[0].Type)
	require.Equal(t, "https://docs.example.com/path/adr.template/index.html", got[0].URL)

	require.Equal(t, "runbook", got[1].Type)
	require.Equal(t, "https://docs.example.com/path/runbook.template/", got[1].URL)
}

func TestParseTemplateDiscovery_NoTemplates(t *testing.T) {
	html := `<html><body><a href="/path/regular/"></a></body></html>`

	_, err := ParseTemplateDiscovery("https://docs.example.com", strings.NewReader(html))
	require.Error(t, err)
}
