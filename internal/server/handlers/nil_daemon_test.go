package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The handlers take the daemon as an interface, and the daemon may be
// missing: the answer says so instead of taking the process down.
func TestTheHandlersAnswerWhenTheDaemonIsNotThere(t *testing.T) {
	build := NewBuildHandlers(nil)
	monitoring := NewMonitoringHandlers(nil)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		// The health check answers that everything is fine even when
		// the daemon is not there: it reports what it knows.
		answersWithoutDaemon bool
	}{
		{name: "the discovery trigger", handler: build.HandleTriggerDiscovery, method: http.MethodPost},
		{name: "the build trigger", handler: build.HandleTriggerBuild, method: http.MethodPost},
		{name: "the build status", handler: build.HandleBuildStatus, method: http.MethodGet},
		{name: "the repositories", handler: build.HandleRepositories, method: http.MethodGet},
		{name: "the health check", handler: monitoring.HandleHealthCheck, method: http.MethodGet, answersWithoutDaemon: true},
		{name: "the metrics", handler: monitoring.HandleMetrics, method: http.MethodGet},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), tc.method, "/", nil)

			tc.handler(rec, req)

			if tc.answersWithoutDaemon {
				if rec.Code != http.StatusOK {
					t.Fatalf("the handler answered %d, it should answer that everything is fine", rec.Code)
				}
				return
			}
			if rec.Code == http.StatusOK {
				t.Fatalf("the handler answered %d, but the daemon is not there", rec.Code)
			}
		})
	}
}
