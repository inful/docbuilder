package daemon

import (
	m "git.home.luguber.info/inful/docbuilder/internal/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

var siteBuildRegistry = prom.NewRegistry()

// resolvePrometheusRecorder returns a Prometheus-backed metrics recorder (always built-in now).
func resolvePrometheusRecorder() m.Recorder { return m.NewPrometheusRecorder(siteBuildRegistry) }
