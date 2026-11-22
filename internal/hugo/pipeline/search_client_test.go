package pipeline

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The client must tell the reader about the query language, but only
// when the reader does not already use it.
func TestTheSearchClientRemindsOnlyOnce(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo:   config.HugoConfig{Title: "Test"},
			Search: &config.SearchConfig{Provider: config.SearchProviderServer},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)
	script := assetContent(t, assets, "assets/js/offline-search.js")

	assert.Contains(t, script, "usesQueryLanguage", "the client must know when to remind")
}

// The reminder is a line of text in the results. It must not be there
// for a reader who already writes the query language.
func TestTheSearchClientRemindsOnlyWhenItHelps(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo:   config.HugoConfig{Title: "Test"},
			Search: &config.SearchConfig{Provider: config.SearchProviderServer},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)
	script := assetContent(t, assets, "assets/js/offline-search.js")

	// The helper is pulled out of the script and asked about the
	// queries of the reader.
	start := strings.Index(script, "const usesQueryLanguage")
	require.GreaterOrEqual(t, start, 0, "the script must hold the helper")
	end := strings.Index(script[start:], "};")
	require.GreaterOrEqual(t, end, 0)
	helper := script[start : start+end+2]

	cases := []struct {
		query string
		want  bool
	}{
		{"install", false},
		{"orange", false},
		{"android", false},
		{"daemon OR kubernetes", true},
		{"daemon AND kubernetes", true},
		{"daemon NOT kubernetes", true},
		{`"package manager"`, true},
		{"title:kubernetes", true},
		{"body:helm", true},
		{"tags:adr", true},
		{"categories:how-to", true},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			program := helper + "\nconsole.log(usesQueryLanguage(" + jsString(tc.query) + "));\n"
			// #nosec G204 -- the program is built from the script of this
			// repository, not from anything the visitor can reach.
			out, err := exec.CommandContext(t.Context(), node, "-e", program).Output()
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(out) == "true\n",
				"usesQueryLanguage(%q) must be %v", tc.query, tc.want)
		})
	}
}

// jsString quotes a string for a JavaScript program.
func jsString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}
