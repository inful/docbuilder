package daemon

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDaemon_schedulePeriodicJobs(t *testing.T) {
	t.Run("errors when scheduler is nil", func(t *testing.T) {
		d := &Daemon{config: &config.Config{Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}}}}
		err := d.schedulePeriodicJobs(context.Background())
		require.Error(t, err)
	})

	t.Run("errors when schedule is empty", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Stop(context.Background()) })

		d := &Daemon{config: &config.Config{Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "  \t "}}}, scheduler: s}
		err = d.schedulePeriodicJobs(context.Background())
		require.Error(t, err)
	})

	t.Run("succeeds for valid schedule", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Stop(context.Background()) })

		cfg := &config.Config{Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}}}
		d := &Daemon{config: cfg, scheduler: s}
		err = d.schedulePeriodicJobs(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, d.syncJobID)
		require.NotEmpty(t, d.statusJobID)
		require.NotEmpty(t, d.promJobID)
	})
}
