package hugo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// The two build paths share the stages after the discovery, so they are
// written once. The order is what the build depends on.
func TestBuildPipeline_StageOrder(t *testing.T) {
	names := func(defs []models.StageDef) []string {
		out := make([]string, 0, len(defs))
		for _, d := range defs {
			out = append(out, string(d.Name))
		}
		return out
	}

	direct := names(buildPipeline(false))
	require.Equal(t, []string{
		"prepare_output",
		"generate_config",
		"copy_content",
		"run_hugo",
		"post_process",
	}, direct)

	full := names(buildPipeline(true))
	require.Equal(t, []string{
		"prepare_output",
		"clone_repos",
		"discover_docs",
		"generate_config",
		"copy_content",
		"run_hugo",
		"post_process",
	}, full)
}
