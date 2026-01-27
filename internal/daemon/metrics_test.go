package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestMetricsCollector_PrometheusHandler_CounterSuffix(t *testing.T) {
	mc := NewMetricsCollector()

	// One counter name already has the conventional suffix.
	mc.IncrementCounter("foo_total")

	// One does not; exporter should add it.
	mc.IncrementCounter("bar")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example/metrics", nil)
	mc.PrometheusHandler(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	require.Contains(t, body, "docbuilder_foo_total 1")
	require.NotContains(t, body, "foo_total_total")

	require.Contains(t, body, "docbuilder_bar_total 1")
	require.NotContains(t, body, "docbuilder_bar_total_total")

	// Sanity-check the output format stays Prometheus-ish.
	require.True(t, strings.Contains(body, "# TYPE docbuilder_foo_total counter"))
}
