package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderOutputPath_WithSequence(t *testing.T) {
	template := `adr/adr-{{ printf "%03d" (nextInSequence "adr") }}-{{ .Slug }}.md`
	data := map[string]any{
		"Slug": "test-decision",
	}

	got, err := RenderOutputPath(template, data, func(name string) (int, error) {
		require.Equal(t, "adr", name)
		return 7, nil
	})
	require.NoError(t, err)
	require.Equal(t, "adr/adr-007-test-decision.md", got)
}
