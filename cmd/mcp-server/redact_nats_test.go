package main

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The URL of NATS holds a password, and it stays out of the answer of
// the server.
//
// The link verification redacts that URL in its own logs, and the
// answer of the get_config tool and of the config resource carried it
// as it is.
func TestTheNATSURLIsRedactedInTheAnswer(t *testing.T) {
	//nolint:gosec // the password of the test is not a credential
	natsURL := "nats://user:secret-password@nats.example.com:4222"

	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			LinkVerification: &config.LinkVerificationConfig{
				NATSURL: natsURL,
			},
		},
	}

	redacted := redactConfig(cfg)

	got := redacted.Daemon.LinkVerification.NATSURL
	if strings.Contains(got, "secret-password") {
		t.Fatalf("the password of the NATS URL is in the answer: %s", got)
	}
	if !strings.Contains(got, "nats.example.com") {
		t.Fatalf("the NATS URL was changed: %s", got)
	}

	// The configuration of the caller is not changed.
	if !strings.Contains(cfg.Daemon.LinkVerification.NATSURL, "secret-password") {
		t.Fatal("the configuration of the caller was changed")
	}
}
