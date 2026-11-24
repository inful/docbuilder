package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// The status of the daemon is part of its state. Nothing wrote it: the
// state file said "starting" for the whole life of the daemon.
func TestTheStatusOfTheDaemonIsWrittenToItsState(t *testing.T) {
	svc := state.NewService(t.TempDir()).Unwrap()
	sm := state.NewServiceAdapter(svc)

	d := daemonForTest(t, daemonForTestOpts{
		Config:       &config.Config{Daemon: &config.DaemonConfig{}},
		StateManager: sm,
	})

	require.Equal(t, StatusRunning, d.GetStatus())

	d.updateStatus(t.Context())

	res := svc.GetDaemonInfoStore().Get(t.Context())
	require.True(t, res.IsOk(), "the daemon info must be readable")
	require.Equal(t, StatusRunning, res.Unwrap().Status)
}
