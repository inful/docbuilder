package linkverify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The page waits for its links.
//
// The hash of the page says that the links of the page are checked: a
// page that is marked as checked while its links are still in the pool
// is never checked again, and the broken links of the page are never
// published.
func TestThePageWaitsForItsLinks(t *testing.T) {
	served := make(chan struct{})
	release := make(chan struct{})
	// The server is released once, also when the test fails: a handler
	// that waits for a release that never comes keeps the test from
	// ending.
	var releaseOnce sync.Once
	releaseServer := func() { releaseOnce.Do(func() { close(release) }) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(served)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	// The cleanups run in the reverse order: the server is released
	// before it is closed, or the close waits for a handler that waits
	// for the release.
	t.Cleanup(srv.Close)
	t.Cleanup(releaseServer)

	cfg := &config.LinkVerificationConfig{Enabled: true, MaxConcurrent: 1}

	// The cache is not reachable: the lookups fail and the page is not
	// skipped.
	nats := &NATSClient{cfg: cfg}
	nats.reconnecting.Store(true)

	svc := &VerificationService{
		cfg:        cfg,
		nats:       nats,
		httpClient: srv.Client(),
	}

	page := &PageMetadata{
		BaseURL:      "https://example.com/",
		RenderedPath: "/page",
		ContentHash:  "hash-of-the-page",
		Links:        []*Link{{URL: srv.URL + "/slow"}},
	}

	// The pool of the links: the worker of the pool checks the link and
	// tells the page that it is done.
	linkQueue := make(chan linkJob)
	linkDone := make(chan struct{})
	go func() {
		defer close(linkDone)
		for job := range linkQueue {
			func() {
				if job.done != nil {
					defer job.done()
				}
				svc.verifyLink(context.Background(), job.link, job.page)
			}()
		}
	}()

	pageDone := make(chan struct{})
	go func() {
		defer close(pageDone)
		svc.verifyPage(context.Background(), page, linkQueue)
	}()

	// The check of the link started.
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the link was never checked")
	}

	// The page waits for its link.
	select {
	case <-pageDone:
		t.Fatal("the page ended before its link was checked")
	case <-time.After(200 * time.Millisecond):
	}

	// The link answers: the page ends.
	releaseServer()
	select {
	case <-pageDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the page did not end")
	}
	close(linkQueue)
	<-linkDone
}
