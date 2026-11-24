package httpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The paths of the monitoring endpoints come from the configuration.
// The router panics on a path without a leading slash and on a path
// that is already served, and the panic takes the whole daemon down:
// the daemon refuses to start with a message instead.
func TestTheAdminServerRefusesAPathThatTheRouterWouldPanicOn(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.MonitoringConfig
	}{
		{
			name: "a health path without a leading slash",
			cfg:  &config.MonitoringConfig{Health: config.MonitoringHealth{Path: "health"}},
		},
		{
			name: "a health path that is already served",
			cfg:  &config.MonitoringConfig{Health: config.MonitoringHealth{Path: "/healthz"}},
		},
		{
			name: "a metrics path that is already served",
			cfg: &config.MonitoringConfig{
				Health:  config.MonitoringHealth{Path: "/health"},
				Metrics: config.MonitoringMetrics{Enabled: true, Path: "/metrics/prometheus"},
			},
		},
		{
			name: "the same path for the health and the metrics endpoint",
			cfg: &config.MonitoringConfig{
				Health:  config.MonitoringHealth{Path: "/same"},
				Metrics: config.MonitoringMetrics{Enabled: true, Path: "/same"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Monitoring: tt.cfg,
				Daemon: &config.DaemonConfig{
					HTTP: config.HTTPConfig{
						DocsPort:    freePort(t),
						WebhookPort: freePort(t),
						AdminPort:   freePort(t),
						AdminBind:   "127.0.0.1",
					},
				},
			}

			srv := New(cfg, &webhookRuntimeStub{}, Options{})
			err := srv.Start(t.Context())
			require.Error(t, err, "the daemon must refuse to start")
		})
	}
}

// The default paths work: the daemon starts.
func TestTheAdminServerStartsWithTheDefaultPaths(t *testing.T) {
	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health:  config.MonitoringHealth{Path: "/health"},
			Metrics: config.MonitoringMetrics{Enabled: true, Path: "/metrics"},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    freePort(t),
				WebhookPort: freePort(t),
				AdminPort:   freePort(t),
				AdminBind:   "127.0.0.1",
			},
		},
	}

	srv := New(cfg, &webhookRuntimeStub{}, Options{})
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(func() { _ = srv.Stop(t.Context()) })
}
