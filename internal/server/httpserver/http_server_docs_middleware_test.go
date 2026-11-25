package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Every route of the docs server answers through the middleware: a
// route that is outside of it gets neither the log nor the answer to a
// panic, and the connection closes without an answer.
func TestEveryRouteOfTheDocsServerAnswersThroughTheMiddleware(t *testing.T) {
	tmpDir := t.TempDir()
	docsPort := freePort(t)

	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health: config.MonitoringHealth{Path: "/health"},
		},
		Output: config.OutputConfig{Directory: tmpDir},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    docsPort,
				WebhookPort: freePort(t),
				AdminPort:   freePort(t),
				AdminBind:   "127.0.0.1",
			},
		},
	}

	srv := New(cfg, testRuntime{}, Options{
		SearchHandle: func(http.ResponseWriter, *http.Request) {
			panic("the search handler broke")
		},
	})
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	url := "http://127.0.0.1:" + strconv.Itoa(docsPort) + "/api/search"
	resp, err := http.DefaultClient.Do(mustRequest(t, url))
	require.NoError(t, err, "the route did not answer at all")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
