package linkverify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The rate limit is documented as "delay between link verification
// requests". It has to sit in front of every request, not in front of the
// page, or a page with many links sends them all at once.
func TestVerifyLinkWaitsTheRateLimitDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.LinkVerificationConfig{Enabled: true, MaxConcurrent: 1}

	// A client that is "reconnecting" answers every cache call with an
	// error right away, so the test does not need a NATS server.
	nats := &NATSClient{cfg: cfg}
	nats.reconnecting.Store(true)

	svc := &VerificationService{
		cfg:        cfg,
		nats:       nats,
		httpClient: srv.Client(),
		semaphore:  make(chan struct{}, 1),
		rateDelay:  150 * time.Millisecond,
	}

	page := &PageMetadata{BaseURL: "https://example.com/"}
	for range 2 {
		svc.semaphore <- struct{}{}
		svc.wg.Add(1)
		start := time.Now()
		svc.verifyLink(context.Background(), &Link{URL: srv.URL + "/page"}, page)
		if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
			t.Fatalf("verifyLink returned after %v, want at least the 150ms rate limit", elapsed)
		}
	}
}
