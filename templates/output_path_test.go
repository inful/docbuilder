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

func TestRenderOutputPath_BuiltinUID(t *testing.T) {
	template := `docs/{{ .UID }}.md`
	got, err := RenderOutputPath(template, map[string]any{}, nil)
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^docs/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.md$`), got)
}

func TestWithBuiltinTemplateData_SharedAcrossRenders(t *testing.T) {
	// The motivating scenario: a single "create from template" call must
	// produce the same UID in the output path AND the body, otherwise the
	// frontmatter uid and a body link won't agree.
	inputs := WithBuiltinTemplateData(map[string]any{"Slug": "x"})

	path, err := RenderOutputPath(`pages/{{ .UID }}-{{ .Slug }}.md`, inputs, nil)
	require.NoError(t, err)

	body, err := RenderTemplateBody(`uid: {{ .UID }}`, inputs, nil)
	require.NoError(t, err)

	uidPattern := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	pathUID := uidPattern.FindString(path)
	bodyUID := uidPattern.FindString(body)

	require.NotEmpty(t, pathUID, "expected UID in rendered path %q", path)
	require.Equal(t, pathUID, bodyUID, "UID must be the same in path and body")
}
