package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
)

// reingestDaemonStub is a daemon that answers the reingestion trigger
// with a job id.
type reingestDaemonStub struct {
	jobID string
}

func (s *reingestDaemonStub) TriggerDiscovery() string { return "" }
func (s *reingestDaemonStub) TriggerBuild() string     { return "" }
func (s *reingestDaemonStub) TriggerReingest() string  { return s.jobID }
func (s *reingestDaemonStub) GetQueueLength() int      { return 0 }
func (s *reingestDaemonStub) GetActiveJobs() int       { return 0 }

func (s *reingestDaemonStub) BuildStats() (responses.BuildStatistics, *responses.BuildInfo) {
	return responses.BuildStatistics{}, nil
}

func (s *reingestDaemonStub) RepositoryStatuses() []responses.RepositoryInfo { return nil }

// The reingestion trigger asks the daemon for a reingestion and answers
// with the id of the build job that carries it.
func TestTheReingestionTriggerAsksTheDaemon(t *testing.T) {
	build := NewBuildHandlers(&reingestDaemonStub{jobID: "reingest-1"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/ragabast/reingest", nil)

	build.HandleTriggerReingest(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "reingest-1")
}

// The trigger only answers to a POST: a GET would move the epoch of the
// ingest without asking for the build that carries the corpus.
func TestTheReingestionTriggerRefusesAnotherMethod(t *testing.T) {
	build := NewBuildHandlers(&reingestDaemonStub{jobID: "reingest-1"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/ragabast/reingest", nil)

	build.HandleTriggerReingest(rec, req)

	require.NotEqual(t, http.StatusOK, rec.Code)
}
