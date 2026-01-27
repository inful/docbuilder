package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsCollector_AddCounter(t *testing.T) {
	mc := NewMetricsCollector()

	mc.AddCounter("x", 3)
	mc.AddCounter("x", 2)
	mc.AddCounter("x", 0)
	mc.AddCounter("x", -5)

	snap := mc.GetSnapshot()
	require.Equal(t, int64(5), snap.Counters["x"])
}
