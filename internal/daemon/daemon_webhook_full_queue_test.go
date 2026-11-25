package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The queue of the consumer is full: the webhook waits for room
// instead of being dropped. The forge does not send it again, and the
// commit it announced was never published.
func TestTheWebhookIsNotDroppedWhenTheQueueIsFull(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	// A consumer that does not read: the queue of the events fills up.
	received, unsubscribe := events.Subscribe[events.WebhookReceived](bus, 1)
	defer unsubscribe()

	d := daemonForTest(t, daemonForTestOpts{
		Config:           &config.Config{Daemon: &config.DaemonConfig{}},
		OrchestrationBus: bus,
	})

	// The first webhook fills the queue.
	require.NotEmpty(t, d.TriggerWebhookBuild("forge", "acme/docs", "main", nil))

	// The second webhook has to wait for room.
	jobID := d.TriggerWebhookBuild("forge", "acme/docs", "main", nil)
	require.NotEmpty(t, jobID, "the webhook was dropped")

	// The consumer reads: the webhook that waited arrives.
	<-received
	select {
	case evt := <-received:
		require.NotEmpty(t, evt.JobID)
	case <-time.After(10 * time.Second):
		t.Fatal("the webhook was not delivered")
	}
}
