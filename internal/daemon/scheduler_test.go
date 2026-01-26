package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduler_ScheduleCron(t *testing.T) {
	t.Run("returns job id for valid cron", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Stop(context.Background()) })

		id, err := s.ScheduleCron("test", "0 */4 * * *", func() {})
		require.NoError(t, err)
		require.NotEmpty(t, id)
	})

	t.Run("rejects invalid cron", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Stop(context.Background()) })

		_, err = s.ScheduleCron("test", "this is not a cron", func() {})
		require.Error(t, err)
	})
}

func TestScheduler_ScheduleEvery(t *testing.T) {
	t.Run("returns job id for valid interval", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Stop(context.Background()) })

		id, err := s.ScheduleEvery("test", 10*time.Second, func() {})
		require.NoError(t, err)
		require.NotEmpty(t, id)
	})

	t.Run("rejects non-positive interval", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Stop(context.Background()) })

		_, err = s.ScheduleEvery("test", 0, func() {})
		require.Error(t, err)
	})
}
