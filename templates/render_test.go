package templates

import (
	"regexp"
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

func TestRenderTemplateBody_BuiltinDate(t *testing.T) {
	body := "Date: {{ .Date }}"
	data := map[string]any{}

	rendered, err := RenderTemplateBody(body, data, nil)
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^Date: \d{4}-\d{2}-\d{2}$`), rendered)
}

func TestRenderTemplateBody_BuiltinUID(t *testing.T) {
	body := "uid: {{ .UID }}"
	rendered, err := RenderTemplateBody(body, map[string]any{}, nil)
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^uid: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`), rendered)
}

func TestRenderTemplateBody_UIDIsSameWithinOneRender(t *testing.T) {
	// If the user references .UID multiple times, every reference must
	// resolve to the same value within one render.
	body := "uid: {{ .UID }}\nalias: /_uid/{{ .UID }}/"
	rendered, err := RenderTemplateBody(body, map[string]any{}, nil)
	require.NoError(t, err)

	// Extract the UID from the first line and verify the alias uses the
	// same value.
	uidPattern := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	matches := uidPattern.FindAllString(rendered, -1)
	require.Len(t, matches, 2, "expected two occurrences of the UID; got %q", rendered)
	require.Equal(t, matches[0], matches[1], "UID references should resolve to the same value")
}
