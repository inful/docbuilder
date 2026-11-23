package daemon

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// The addresses of the daemon hold credentials.
//
// The URL of NATS holds a password, and the address of the ragabast can
// hold credentials: they were logged as they are, and the log line ends
// up in a log collector.
func TestTheAddressesOfTheDaemonAreRedacted(t *testing.T) {
	//nolint:gosec // the password of the test is not a credential
	raw := "nats://bot:s3cr3t@nats.internal:4222"

	redacted := logfields.RedactURL(raw)

	if strings.Contains(redacted, "s3cr3t") {
		t.Fatalf("the password of the address is in the log: %s", redacted)
	}
	if !strings.Contains(redacted, "nats.internal") {
		t.Fatalf("the address was changed: %s", redacted)
	}
}

// The dispatcher prints its address without the credentials.
func TestTheDispatcherPrintsItsAddressWithoutTheCredentials(t *testing.T) {
	//nolint:gosec // the password of the test is not a credential
	baseURL := "https://user:secret@ragabast.example.com"
	dispatcher := &OutboundDispatcher{
		baseURL: baseURL,
	}

	printed := dispatcher.String()

	if strings.Contains(printed, "secret") {
		t.Fatalf("the password of the address is in the text: %s", printed)
	}
}
