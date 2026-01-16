package linkverify

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

type inMemoryCache struct {
	mu       sync.Mutex
	links    map[string]*CacheEntry
	pageHash map[string]string
}

func newInMemoryCache() *inMemoryCache {
	return &inMemoryCache{
		links:    make(map[string]*CacheEntry),
		pageHash: make(map[string]string),
	}
}

func (c *inMemoryCache) GetCachedResult(_ context.Context, url string) (*CacheEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.links[url]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, ErrCacheMiss
}

func (c *inMemoryCache) SetCachedResult(_ context.Context, entry *CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry == nil {
		return nil
	}
	cp := *entry
	c.links[entry.URL] = &cp
	return nil
}

func (c *inMemoryCache) IsCacheValid(_ *CacheEntry) bool { return false }

func (c *inMemoryCache) GetPageHash(_ context.Context, path string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.pageHash[path]; ok {
		return v, nil
	}
	return "", ErrCacheMiss
}

func (c *inMemoryCache) SetPageHash(_ context.Context, path string, hash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pageHash[path] = hash
	return nil
}

func (c *inMemoryCache) PublishBrokenLink(_ context.Context, _ *BrokenLinkEvent) error { return nil }

func (c *inMemoryCache) Close() error { return nil }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerificationService_VerifyPages_Completes(t *testing.T) {
	tmp := t.TempDir()
	htmlPath := filepath.Join(tmp, "page.html")

	// Create HTML with many links to exercise concurrency.
	var b strings.Builder
	b.WriteString("<html><body>")
	for range 200 {
		b.WriteString("<a href=\"https://example.com/x\">x</a>")
	}
	b.WriteString("</body></html>")
	if err := os.WriteFile(htmlPath, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write html: %v", err)
	}

	cfg := &config.LinkVerificationConfig{
		Enabled:        true,
		MaxConcurrent:  5,
		RequestTimeout: "2s",
		RateLimitDelay: "0s",
		MaxRedirects:   3,
	}

	svc := &VerificationService{
		cfg:   cfg,
		cache: newInMemoryCache(),
		httpClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: http.NoBody}, nil
		})},
		linkSem: make(chan struct{}, cfg.MaxConcurrent),
		pageSem: make(chan struct{}, min(cfg.MaxConcurrent, 4)),
	}

	pages := []*PageMetadata{
		{HTMLPath: htmlPath, RenderedPath: "page.html", BaseURL: "https://example.com/", ContentHash: ""},
		{HTMLPath: htmlPath, RenderedPath: "page2.html", BaseURL: "https://example.com/", ContentHash: ""},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.VerifyPages(ctx, pages); err != nil {
		t.Fatalf("VerifyPages: %v", err)
	}

	svc.mu.Lock()
	running := svc.running
	svc.mu.Unlock()
	if running {
		t.Fatalf("expected verification not running after completion")
	}
}
