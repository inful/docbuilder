package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/forge/discoveryrunner"
)

// stubHTTPServer stands in for the HTTP server of the daemon.
//
// It watches the bus at the moment the server starts: that is the
// moment the daemon accepts the requests.
type stubHTTPServer struct {
	onStart func()
}

func (s *stubHTTPServer) Start(context.Context) error {
	if s.onStart != nil {
		s.onStart()
	}
	return nil
}

func (s *stubHTTPServer) Stop(context.Context) error { return nil }

// The bus has no queue: an event that is published before the consumer
// is subscribed is lost. The server accepted the requests before the
// consumers were subscribed, and a webhook that arrived in that window
// was thrown away.
func TestTheConsumersAreSubscribedWhenTheServerStarts(t *testing.T) {
	scheduler, err := NewScheduler()
	require.NoError(t, err)

	cfg := &config.Config{Daemon: &config.DaemonConfig{
		Sync: config.SyncConfig{Schedule: "0 */4 * * *"},
	}}

	runner := NewDiscoveryRunner(DiscoveryRunnerConfig{
		Discovery:      &fakeDiscovery{result: &forge.DiscoveryResult{}},
		DiscoveryCache: discoveryrunner.NewCache(),
		Config:         cfg,
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return "job-1" },
	})

	d := daemonForTest(t, daemonForTestOpts{
		Config:          cfg,
		StateManager:    newFakeDaemonStateManager(),
		Scheduler:       scheduler,
		DiscoveryRunner: runner,
	})
	d.serve.metrics = NewMetricsCollector()
	d.build.buildQueue = queue.NewBuildQueue(4, 1, &stubBuilder{})

	// The stub watches the bus when the server starts.
	var subscribed atomic.Int64
	var started atomic.Bool
	d.serve.httpServer = &stubHTTPServer{onStart: func() {
		subscribed.Store(int64(events.SubscriberCount[events.WebhookReceived](d.serve.orchestrationBus)))
		started.Store(true)
	}}

	d.lifecycle.status.Store(StatusStopped)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = d.Start(ctx) }()

	require.Eventually(t, started.Load,
		5*time.Second, 5*time.Millisecond, "the daemon did not start")

	require.Equal(t, int64(1), subscribed.Load(),
		"the consumers of the bus are not subscribed when the server starts")
}
