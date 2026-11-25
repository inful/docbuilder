package build

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A build without repositories is a mistake, and the service refuses
// it. The build that follows the removal of the last repository is not
// a mistake: the pages of the repository must go away, and only a build
// takes them away.
func TestTheBuildOfARemovedRepositoryIsNotRefused(t *testing.T) {
	svc := NewBuildService()

	result, err := svc.Run(t.Context(), BuildRequest{
		Config:    &config.Config{},
		OutputDir: t.TempDir(),
		Removal:   true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Report, "the build did not run: the pages of the removed repository stay")
}

// A build without repositories and without a removal is refused: the
// operator forgot to name the repositories.
func TestTheBuildWithoutARepositoryIsRefused(t *testing.T) {
	svc := NewBuildService()

	result, err := svc.Run(t.Context(), BuildRequest{
		Config:    &config.Config{},
		OutputDir: t.TempDir(),
	})
	require.NoError(t, err)
	require.Nil(t, result.Report, "the build ran without a repository")
}
