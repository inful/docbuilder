package linkverify

import (
	"context"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The attempts to connect are spaced out.
//
// The cache of every link of every page asked for a connection, and a
// server that is down was asked thousands of times in one build.
func TestTheReconnectIsSpacedOut(t *testing.T) {
	cfg := &config.LinkVerificationConfig{
		Enabled: true,
		NATSURL: "nats://127.0.0.1:1",
	}
	client := &NATSClient{cfg: cfg}

	// The first attempt fails: there is no server there.
	if err := client.ensureConnected(context.Background()); err == nil {
		t.Fatal("the connection to a server that is not there succeeded")
	}

	// The second attempt is not made: the attempts are spaced out.
	err := client.ensureConnected(context.Background())
	if err == nil {
		t.Fatal("the client connected a second time right away")
	}
	if !strings.Contains(err.Error(), "not retried") {
		t.Fatalf("the client tried to connect again: %v", err)
	}
}
