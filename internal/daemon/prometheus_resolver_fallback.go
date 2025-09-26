//go:build !prometheus

package daemon

import m "git.home.luguber.info/inful/docbuilder/internal/metrics"

// resolvePrometheusRecorder returns nil when prometheus tag not set.
func resolvePrometheusRecorder() m.Recorder { return nil }
