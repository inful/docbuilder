package templates

import (
	"regexp"
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

func TestRenderOutputPath_BuiltinDate(t *testing.T) {
	template := `daily/{{ .Date }}.md`
	got, err := RenderOutputPath(template, map[string]any{}, nil)
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^daily/\d{4}-\d{2}-\d{2}\.md$`), got)
}
