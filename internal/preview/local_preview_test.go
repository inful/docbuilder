package preview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestValidateAndResolveDocsDir_RequiresRepository(t *testing.T) {
	cfg := &config.Config{}
	_, err := validateAndResolveDocsDir(cfg)
	require.Error(t, err)
}

func TestValidateAndResolveDocsDir_ErrorsWhenMissingDir(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{URL: t.TempDir() + "/does-not-exist"}}}
	_, err := validateAndResolveDocsDir(cfg)
	require.Error(t, err)
}

func TestValidateAndResolveDocsDir_ReturnsAbsoluteDir(t *testing.T) {
	docsDir := t.TempDir()
	cfg := &config.Config{Repositories: []config.Repository{{URL: docsDir}}}

	abs, err := validateAndResolveDocsDir(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, abs)
	require.True(t, filepath.IsAbs(abs))
}

func TestShouldIgnoreEvent(t *testing.T) {
	require.True(t, shouldIgnoreEvent("/tmp/.hidden.md"))
	require.True(t, shouldIgnoreEvent("/tmp/#foo#"))
	require.True(t, shouldIgnoreEvent("/tmp/foo.swp"))
	require.True(t, shouldIgnoreEvent("/tmp/.DS_Store"))
	require.False(t, shouldIgnoreEvent("/tmp/visible.md"))
}

func TestEnsureVSCodeMarkdownSnippetSettings_CreatesFile(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), ".vscode", "settings.json")

	err := ensureVSCodeMarkdownSnippetSettings(settingsPath)
	require.NoError(t, err)

	// #nosec G304 -- test reads from a temporary file path controlled by the test.
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	md, ok := settings["[markdown]"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "top", md["editor.snippetSuggestions"])
	require.Equal(t, true, md["editor.suggest.showSnippets"])
	require.Equal(t, "off", md["editor.wordBasedSuggestions"])

	quick, ok := md["editor.quickSuggestions"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, quick["other"])
	require.Equal(t, false, quick["comments"])
	require.Equal(t, true, quick["strings"])
}

func TestEnsureVSCodeMarkdownSnippetSettings_MergesExistingSettings(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".vscode", "settings.json")

	seed := map[string]any{
		"makefile.configureOnOpen": false,
		"[markdown]": map[string]any{
			"editor.formatOnSave": true,
		},
	}
	data, err := json.Marshal(seed)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o750))
	require.NoError(t, os.WriteFile(settingsPath, data, 0o600))

	err = ensureVSCodeMarkdownSnippetSettings(settingsPath)
	require.NoError(t, err)

	// #nosec G304 -- test reads from a temporary file path controlled by the test.
	resultData, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(resultData, &settings))

	require.Equal(t, false, settings["makefile.configureOnOpen"])

	md, ok := settings["[markdown]"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, md["editor.formatOnSave"])
	require.Equal(t, "top", md["editor.snippetSuggestions"])
	require.Equal(t, true, md["editor.suggest.showSnippets"])
	require.Equal(t, "off", md["editor.wordBasedSuggestions"])
}
