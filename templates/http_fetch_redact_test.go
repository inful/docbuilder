package templates

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The address of a page can hold a token, and it is not in the error
// messages.
func TestTheErrorOfAFetchDoesNotHoldTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse the address of the server: %v", err)
	}
	parsed.User = url.UserPassword("user", "secret-token")

	_, err = FetchTemplatePage(t.Context(), parsed.String(), NewTemplateHTTPClient())
	if err == nil {
		t.Fatal("the fetch of a page that fails did not fail")
	}

	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("the token of the address is in the error: %s", err)
	}
	if !strings.Contains(err.Error(), parsed.Host) {
		t.Fatalf("the address was changed: %s", err)
	}
}
