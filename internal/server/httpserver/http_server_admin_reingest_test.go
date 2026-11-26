package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// reingestRuntimeStub answers the reingestion trigger with a job id.
type reingestRuntimeStub struct {
	webhookRuntimeStub
}

func (r *reingestRuntimeStub) TriggerReingest() string { return "reingest-1" }

// The admin server serves the reingestion trigger: the operator asks
// for the corpus to be sent to ragabast again.
func TestTheAdminServerServesTheReingestionTrigger(t *testing.T) {
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

	srv := New(cfg, &reingestRuntimeStub{}, Options{})
	mux := srv.adminMux()
	require.NotNil(t, mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/ragabast/reingest", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "reingest-1")
}
