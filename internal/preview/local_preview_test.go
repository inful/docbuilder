package preview

import (
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
