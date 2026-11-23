package commands

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The flag decides where the daemon keeps its state. It used to be dead:
// the daemon only logged it and always wrote its state to the state_dir of
// the configuration, so `docbuilder daemon -d ./elsewhere` wrote its state
// somewhere else entirely.
func TestTheDataDirFlagDecidesWhereTheStateLives(t *testing.T) {
	cfg := &config.Config{Daemon: &config.DaemonConfig{}}
	cfg.Daemon.Storage.StateDir = "./from-the-config"

	applyDataDir(cfg, "./from-the-flag")

	if cfg.Daemon.Storage.StateDir != "./from-the-flag" {
		t.Fatalf("the flag did not decide: %s", cfg.Daemon.Storage.StateDir)
	}
}

// Without the flag the configuration decides: a deployment that sets a
// state_dir keeps it.
func TestTheConfigurationDecidesWhenTheFlagIsAbsent(t *testing.T) {
	cfg := &config.Config{Daemon: &config.DaemonConfig{}}
	cfg.Daemon.Storage.StateDir = "./from-the-config"

	applyDataDir(cfg, "")

	if cfg.Daemon.Storage.StateDir != "./from-the-config" {
		t.Fatalf("the configuration did not decide: %s", cfg.Daemon.Storage.StateDir)
	}
}
