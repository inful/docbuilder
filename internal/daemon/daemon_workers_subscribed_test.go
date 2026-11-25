package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The bus has no queue: an event that is published before the consumer
// is subscribed is lost. The server accepts the requests before the
// consumers are subscribed, and a webhook that arrived in that window
// was thrown away.
func TestTheConsumersAreSubscribedWhenTheWorkersStart(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	d := daemonForTest(t, daemonForTestOpts{
		Config:           &config.Config{Daemon: &config.DaemonConfig{}},
		OrchestrationBus: bus,
	})

	d.startWorkers(t.Context())

	require.Equal(t, 1, events.SubscriberCount[events.WebhookReceived](bus),
		"the consumer of the webhooks is not subscribed")
	require.Equal(t, 1, events.SubscriberCount[events.RepoRemoved](bus),
		"the consumer of the removals is not subscribed")
	require.Equal(t, 1, events.SubscriberCount[events.BuildNow](bus),
		"the consumer of the builds is not subscribed")

	// The events that are published now are not lost.
	require.NoError(t, bus.Publish(t.Context(), events.WebhookReceived{JobID: "job-1"}))
	require.NoError(t, bus.Publish(t.Context(), events.RepoRemoved{RepoURL: "https://example.invalid/repo.git"}))

	time.Sleep(50 * time.Millisecond)
}
