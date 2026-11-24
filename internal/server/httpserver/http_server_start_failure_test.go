package httpserver

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A server that fails to start leaves the ports of the servers that
// did not start bound: the daemon would hold a port that nobody
// answers on.
func TestThePortsAreReleasedWhenAServerFailsToStart(t *testing.T) {
	docsPort := freePort(t)
	webhookPort := freePort(t)
	adminPort := freePort(t)

	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health: config.MonitoringHealth{Path: "/health"},
		},
		// A webhook without a secret is refused: the webhook server
		// does not start.
		Forges: []*config.ForgeConfig{
			{
				Name: "company-github",
				Type: config.ForgeGitHub,
				Webhook: &config.WebhookConfig{
					Path: "/webhooks/github",
				},
			},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    docsPort,
				WebhookPort: webhookPort,
				AdminPort:   adminPort,
				AdminBind:   "127.0.0.1",
			},
		},
	}

	srv := New(cfg, &webhookRuntimeStub{}, Options{})
	err := srv.Start(t.Context())
	require.Error(t, err, "the webhook server has no secret and must not start")

	// The ports are free again: the daemon can start once more.
	for name, port := range map[string]int{"docs": docsPort, "webhook": webhookPort, "admin": adminPort} {
		lc := net.ListenConfig{}
		ln, lerr := lc.Listen(t.Context(), "tcp", net.JoinHostPort("", strconv.Itoa(port)))
		require.NoError(t, lerr, "the %s port is still bound", name)
		_ = ln.Close()
	}
}
