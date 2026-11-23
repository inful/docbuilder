package templates

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// The address of a page can hold a token, and it is not in the error
// messages.
func TestTheErrorOfAFetchDoesNotHoldTheToken(t *testing.T) {
	//nolint:gosec // the token of the test is not a credential
	pageURL := "https://user:secret-token@docs.example.com/page.template/"

	redacted := logfields.RedactURL(pageURL)

	if strings.Contains(redacted, "secret-token") {
		t.Fatalf("the token of the address is in the error: %s", redacted)
	}
	if !strings.Contains(redacted, "docs.example.com") {
		t.Fatalf("the address was changed: %s", redacted)
	}
}
