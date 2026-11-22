package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The daemon must keep its state in its own directory, not in the
// repository cache: sharing one directory meant every state save walked
// every cloned repository.
func TestDaemon_StateDirSeparateFromRepoCache(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			Storage: config.StorageConfig{
				RepoCacheDir:   filepath.Join(root, "cache"),
				StateDir:       filepath.Join(root, "state"),
				EventRetention: "720h",
			},
		},
		Build:  config.BuildConfig{WorkspaceDir: filepath.Join(root, "ws")},
		Output: config.OutputConfig{Directory: filepath.Join(root, "site")},
	}
	if err := os.MkdirAll(cfg.Daemon.Storage.RepoCacheDir, 0o755); err != nil {
		t.Fatalf("mkdir repo cache: %v", err)
	}

	d := daemonForTest(t, daemonForTestOpts{Config: cfg})
	if err := d.initStateStack(cfg); err != nil {
		t.Fatalf("initStateStack: %v", err)
	}

	if _, err := os.Stat(cfg.Daemon.Storage.StateDir); err != nil {
		t.Errorf("expected the state directory to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Daemon.Storage.RepoCacheDir, "daemon-state.json")); err == nil {
		t.Error("the state file must not live in the repository cache")
	}
}
