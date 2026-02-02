package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderTemplateBody(t *testing.T) {
	body := "Title: {{ .Title }}\nNumber: {{ nextInSequence \"adr\" }}"
	data := map[string]any{
		"Title": "Example",
	}

	rendered, err := RenderTemplateBody(body, data, func(name string) (int, error) {
		require.Equal(t, "adr", name)
		return 2, nil
	})
	require.NoError(t, err)
	require.Equal(t, "Title: Example\nNumber: 2", rendered)
}
