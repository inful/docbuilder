package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A document whose preflight failed with 401 was not uploaded. It must
// not be remembered as uploaded: the next build would skip it for
// ever, even after the auth is fixed.
func TestADocumentThatWasNotUploadedIsNotRememberedAsUploaded(t *testing.T) {
	authBroken := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// The upload succeeds.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if authBroken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// The preflight: the document is not there yet.
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL:   srv.URL,
		Token:     "t",
		Workers:   1,
		QueueSize: 4,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	content := []byte("---\nuid: u1\nfingerprint: abc123\n---\n\n# Doc\n")

	// The auth is broken: the document is not uploaded.
	d.send(context.Background(), ingestJob{content: content, path: "doc.md"})

	if uploaded, ok := d.uploadedFingerprint("u1"); ok {
		t.Fatalf("the document was remembered as uploaded: %s", uploaded)
	}

	// The auth is fixed: the document is uploaded.
	authBroken = false
	d.send(context.Background(), ingestJob{content: content, path: "doc.md"})

	if uploaded, ok := d.uploadedFingerprint("u1"); !ok || uploaded != "abc123" {
		t.Fatalf("the document was not uploaded after the auth was fixed: %v %v", uploaded, ok)
	}
}
