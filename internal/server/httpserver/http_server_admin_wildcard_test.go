package httpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The router panics on a path with a broken wildcard: one character of a
// typo in the configuration took the whole daemon down. The daemon
// refuses to start with a message instead.
func TestTheAdminServerRefusesAPathWithAWildcard(t *testing.T) {
	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health: config.MonitoringHealth{Path: "/heal{th"},
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

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the daemon panicked on the monitoring path: %v", r)
		}
	}()

	err := srv.Start(t.Context())
	require.Error(t, err, "the daemon must refuse to start")
	require.Contains(t, err.Error(), "wildcard")
}
