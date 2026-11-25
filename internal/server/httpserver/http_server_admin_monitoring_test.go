package httpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The admin server serves the monitoring endpoints, and the paths of
// them come from the configuration: a configuration without them took
// the process down when the server read the path.
func TestTheAdminServerRefusesAConfigurationWithoutTheMonitoringPaths(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.MonitoringConfig
	}{
		{name: "no monitoring configuration", cfg: nil},
		{name: "an empty health path", cfg: &config.MonitoringConfig{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Monitoring: tc.cfg,
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

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("the daemon panicked on the monitoring configuration: %v", r)
				}
			}()

			err := srv.Start(t.Context())
			require.Error(t, err, "the daemon must refuse to start")
		})
	}
}
