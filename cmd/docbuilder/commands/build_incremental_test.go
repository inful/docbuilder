package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The incremental build needs a workspace that stays: the clone of the
// repository has to be there, or the build clones every repository
// again and the flag does nothing.
func TestTheIncrementalBuildUsesAWorkspaceThatStays(t *testing.T) {
	wsDir := t.TempDir()
	cfg := &config.Config{
		Build: config.BuildConfig{WorkspaceDir: wsDir},
		// A directory that is not a repository: the clone fails, and
		// the workspace was made before that.
		Repositories: []config.Repository{{Name: "test", URL: filepath.Join(t.TempDir(), "repo")}},
	}

	_ = RunBuild(cfg, filepath.Join(t.TempDir(), "site"), true, false, false)

	if _, err := os.Stat(filepath.Join(wsDir, "working")); err != nil {
		t.Fatalf("the workspace of the incremental build was removed: %v", err)
	}
}

// A build that is not incremental throws its workspace away: the clone
// of the repository is of no use to the next build.
func TestTheBuildWithoutIncrementalThrowsItsWorkspaceAway(t *testing.T) {
	wsDir := t.TempDir()
	cfg := &config.Config{
		Build:        config.BuildConfig{WorkspaceDir: wsDir},
		Repositories: []config.Repository{{Name: "test", URL: filepath.Join(t.TempDir(), "repo")}},
	}

	_ = RunBuild(cfg, filepath.Join(t.TempDir(), "site"), false, false, false)

	entries, err := os.ReadDir(wsDir)
	if err != nil {
		t.Fatalf("read the workspace dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("the workspace of the build was kept: %v", entries)
	}
}
