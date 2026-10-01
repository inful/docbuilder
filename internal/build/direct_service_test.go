package build

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	testforge "git.home.luguber.info/inful/docbuilder/internal/testutil/testforge"
)

// TestDefaultBuildService_RunDirect_BasicSuccess verifies RunDirect accepts
// already-discovered doc files (the preview / build -d scenario) and produces
// the same report shape as Run, without going through clone+discovery.
func TestDefaultBuildService_RunDirect_BasicSuccess(t *testing.T) {
	outDir := t.TempDir()

	forge := testforge.NewTestForge("run-direct-test", config.ForgeGitHub)
	repos := forge.ToConfigRepositories()
	require.NotEmpty(t, repos, "TestForge must produce at least one repository")

	cfg := &config.Config{
		Hugo:  config.HugoConfig{Title: "RunDirect Test"},
		Build: config.BuildConfig{RenderMode: config.RenderModeNever},
	}

	files := []docs.DocFile{{
		Repository:   repos[0].Name,
		Name:         "intro",
		RelativePath: "intro.md",
		DocsBase:     "docs",
		Extension:    ".md",
		Content:      []byte("# RunDirect\n\nDirect-path test content."),
	}}

	svc := NewBuildService()

	result, err := svc.RunDirect(t.Context(), DirectBuildRequest{
		Config:    cfg,
		OutputDir: outDir,
		DocFiles:  files,
	})

	require.NoError(t, err, "RunDirect failed")
	require.Equal(t, BuildStatusSuccess, result.Status, "expected successful build")
	require.NotNil(t, result.Report, "expected a build report")
	require.Equal(t, 1, result.FilesProcessed, "expected one file processed")
	require.False(t, result.Skipped, "direct path should not be skipped")
}

// TestDefaultBuildService_RunDirect_NilConfig verifies RunDirect validates
// its inputs the same way Run does.
func TestDefaultBuildService_RunDirect_NilConfig(t *testing.T) {
	svc := NewBuildService()

	result, err := svc.RunDirect(t.Context(), DirectBuildRequest{
		Config:    nil,
		OutputDir: t.TempDir(),
	})

	require.Error(t, err, "expected error for nil config")
	require.Equal(t, BuildStatusFailed, result.Status, "expected failed status")
}
