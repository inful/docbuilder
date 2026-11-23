package linkverify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// A page without a document file must not take the daemon down. The
// broken-link event is published with empty source fields instead.
func TestVerifyLinkWithoutDocFileDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.LinkVerificationConfig{
		Enabled:          true,
		NATSURL:          "nats://127.0.0.1:1", // no server: the cache is skipped
		Subject:          "x",
		KVBucket:         "b",
		CacheTTL:         "1h",
		CacheTTLFailures: "1h",
		MaxConcurrent:    1,
		RequestTimeout:   "1s",
		RateLimitDelay:   "1ms",
	}

	// A client that is "reconnecting" answers every cache call with an
	// error right away, so the test does not need a NATS server.
	nats := &NATSClient{cfg: cfg}
	nats.reconnecting.Store(true)

	svc := &VerificationService{
		cfg:        cfg,
		nats:       nats,
		httpClient: srv.Client(),
		rateDelay:  time.Millisecond,
	}

	page := &PageMetadata{
		RenderedPath: "page.html",
		BaseURL:      srv.URL + "/",
		Links:        []*Link{{URL: srv.URL + "/missing"}},
	}

	svc.verifyLink(context.Background(), page.Links[0], page)

	// The same page with a document file still fills the source fields.
	page.DocFile = &docs.DocFile{Path: "/docs/a.md", Repository: "repo"}
	if got := docField(page, func(d *docs.DocFile) string { return d.Path }); got != "/docs/a.md" {
		t.Fatalf("path = %q, want /docs/a.md", got)
	}
}
