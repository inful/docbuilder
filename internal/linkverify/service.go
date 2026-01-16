package linkverify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// ErrNoFrontMatter is returned when content has no front matter.
var ErrNoFrontMatter = errors.New("no front matter")

// PageMetadata contains metadata about a page for verification.
type PageMetadata struct {
	DocFile      *docs.DocFile  // Original doc file reference
	HTMLPath     string         // Path to rendered HTML file
	HugoPath     string         // Hugo content path
	RenderedPath string         // Path in rendered site
	RenderedURL  string         // Full URL of rendered page
	FrontMatter  map[string]any // Parsed front matter
	BaseURL      string         // Site base URL
	BuildID      string         // Build identifier
	BuildTime    time.Time      // Build timestamp
	ContentHash  string         // MD5 hash of page content for change detection
}

// VerificationService manages link verification operations.
type VerificationService struct {
	cfg        *config.LinkVerificationConfig
	cache      cacheClient
	httpClient *http.Client
	mu         sync.Mutex
	running    bool
	linkSem    chan struct{} // Limit concurrent link checks
	pageSem    chan struct{} // Limit concurrent page processing
}

type cacheClient interface {
	GetCachedResult(ctx context.Context, url string) (*CacheEntry, error)
	SetCachedResult(ctx context.Context, entry *CacheEntry) error
	IsCacheValid(entry *CacheEntry) bool
	GetPageHash(ctx context.Context, path string) (string, error)
	SetPageHash(ctx context.Context, path string, hash string) error
	PublishBrokenLink(ctx context.Context, event *BrokenLinkEvent) error
	Close() error
}

// NewVerificationService creates a new link verification service.
func NewVerificationService(cfg *config.LinkVerificationConfig) (*VerificationService, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("link verification is disabled")
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 10
	}

	// Create NATS client
	natsClient, err := NewNATSClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS client: %w", err)
	}

	// Parse timeout
	timeout, err := time.ParseDuration(cfg.RequestTimeout)
	if err != nil {
		timeout = 10 * time.Second
	}

	// Create HTTP transport with proxy support
	// This respects HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Create HTTP client with timeout and proxy support
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !cfg.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
			}
			return nil
		},
	}

	return &VerificationService{
		cfg:        cfg,
		cache:      natsClient,
		httpClient: httpClient,
		linkSem:    make(chan struct{}, cfg.MaxConcurrent),
		pageSem:    make(chan struct{}, min(cfg.MaxConcurrent, 4)),
	}, nil
}

// VerifyPages verifies all links in the given pages.
func (s *VerificationService) VerifyPages(ctx context.Context, pages []*PageMetadata) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("verification already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	slog.Info("Starting link verification", "page_count", len(pages))

	// Parse rate limit delay
	delay, err := time.ParseDuration(s.cfg.RateLimitDelay)
	if err != nil {
		delay = 100 * time.Millisecond
	}

	var pagesWG sync.WaitGroup
	for _, page := range pages {
		select {
		case <-ctx.Done():
			slog.Info("Link verification canceled")
			pagesWG.Wait()
			return ctx.Err()
		default:
		}

		// Rate limiting
		time.Sleep(delay)

		// Verify page in background (bounded)
		select {
		case <-ctx.Done():
			pagesWG.Wait()
			return ctx.Err()
		case s.pageSem <- struct{}{}:
		}
		pagesWG.Add(1)
		go func(page *PageMetadata) {
			defer pagesWG.Done()
			defer func() { <-s.pageSem }()
			s.verifyPage(ctx, page)
		}(page)
	}

	// Wait for all verifications to complete
	pagesWG.Wait()
	slog.Info("Link verification completed")

	return nil
}

// verifyPage verifies all links in a single page.
func (s *VerificationService) verifyPage(ctx context.Context, page *PageMetadata) {
	// Check if page content has changed using MD5 hash
	if page.ContentHash != "" {
		if cachedHash, err := s.cache.GetPageHash(ctx, page.RenderedPath); err == nil && cachedHash == page.ContentHash {
			slog.Debug("Skipping link verification for unchanged page",
				"path", page.RenderedPath,
				"hash", page.ContentHash[:8])
			return
		}
	}

	// Extract links from HTML
	links, err := ExtractLinks(page.HTMLPath, page.BaseURL)
	if err != nil {
		slog.Warn("Failed to extract links from page",
			"path", page.HTMLPath,
			"error", err)
		return
	}

	// Filter links based on configuration
	includeInternal := !s.cfg.VerifyExternalOnly
	includeExternal := true
	links = FilterLinks(links, includeInternal, includeExternal)

	slog.Debug("Extracted links from page",
		"path", page.RenderedPath,
		"link_count", len(links))

	// Verify each link
	var linksWG sync.WaitGroup
	for _, link := range links {
		select {
		case <-ctx.Done():
			linksWG.Wait()
			return
		default:
		}

		if !ShouldVerifyLink(link) {
			continue
		}

		// Skip edit links if configured
		if s.cfg.SkipEditLinks && isEditLink(link.URL) {
			continue
		}

		// Acquire link semaphore before spawning to avoid goroutine backlogs.
		select {
		case <-ctx.Done():
			linksWG.Wait()
			return
		case s.linkSem <- struct{}{}:
		}
		linksWG.Add(1)
		go func(link *Link) {
			defer linksWG.Done()
			defer func() { <-s.linkSem }()
			s.verifyLink(ctx, link, page)
		}(link)
	}

	linksWG.Wait()

	// All links verified for this page - update page hash in cache
	if page.ContentHash != "" {
		if err := s.cache.SetPageHash(ctx, page.RenderedPath, page.ContentHash); err != nil {
			slog.Debug("Failed to cache page hash", "path", page.RenderedPath, "error", err)
		}
	}
}

// verifyLink verifies a single link.
func (s *VerificationService) verifyLink(ctx context.Context, link *Link, page *PageMetadata) {
	// Resolve relative URLs
	absoluteURL := link.URL
	if link.IsInternal && !strings.HasPrefix(link.URL, "http") {
		base, err := url.Parse(page.BaseURL)
		if err == nil {
			rel, err := url.Parse(link.URL)
			if err == nil {
				absoluteURL = base.ResolveReference(rel).String()
			}
		}
	}

	// Check cache first
	cached, err := s.cache.GetCachedResult(ctx, absoluteURL)
	if err != nil {
		slog.Debug("Cache lookup error", "url", absoluteURL, "error", err)
	} else if cached != nil && s.cache.IsCacheValid(cached) {
		// Use cached result
		if !cached.IsValid {
			// Still broken, update failure count and possibly publish
			s.handleBrokenLink(ctx, absoluteURL, link, page, cached.Status, cached.Error, cached)
		}
		return
	}

	// Verify the link
	status, verifyErr := s.checkLink(ctx, absoluteURL, link.IsInternal, page)

	// Create cache entry
	cacheEntry := &CacheEntry{
		URL:         absoluteURL,
		Status:      status,
		IsValid:     verifyErr == nil,
		LastChecked: time.Now(),
	}

	if verifyErr != nil {
		cacheEntry.Error = verifyErr.Error()
		s.updateFailureTracking(cacheEntry, cached)
		s.handleBrokenLink(ctx, absoluteURL, link, page, status, verifyErr.Error(), cacheEntry)
	} else {
		// Link is valid, reset failure count
		cacheEntry.FailureCount = 0
		cacheEntry.ConsecutiveFail = false
	}

	// Update cache
	if err := s.cache.SetCachedResult(ctx, cacheEntry); err != nil {
		slog.Warn("Failed to update cache", "url", absoluteURL, "error", err)
	}
}

// checkLink performs the actual link verification.
// updateFailureTracking updates the failure count and timing for a failed link.
func (s *VerificationService) updateFailureTracking(entry *CacheEntry, cached *CacheEntry) {
	if cached != nil {
		entry.FailureCount = cached.FailureCount + 1
		entry.FirstFailedAt = cached.FirstFailedAt
		if entry.FirstFailedAt.IsZero() {
			entry.FirstFailedAt = time.Now()
		}
	} else {
		entry.FailureCount = 1
		entry.FirstFailedAt = time.Now()
	}
	entry.ConsecutiveFail = true
}

// The linkURL should already be an absolute URL, resolved by verifyLink.
func (s *VerificationService) checkLink(ctx context.Context, linkURL string, isInternal bool, page *PageMetadata) (int, error) {
	slog.Debug("Checking link",
		"url", linkURL,
		"is_internal", isInternal,
		"base_url", page.BaseURL)

	return s.checkExternalLink(ctx, linkURL)
}

// checkExternalLink verifies an external link via HTTP request.
func (s *VerificationService) checkExternalLink(ctx context.Context, linkURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, linkURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "DocBuilder-LinkVerifier/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close errors after reading
	}()

	// Discard body
	_, _ = io.Copy(io.Discard, resp.Body)

	// Treat authentication/authorization errors as valid links
	// These indicate the URL exists but requires credentials
	if isAuthError(resp.StatusCode) {
		return resp.StatusCode, nil
	}

	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.StatusCode, nil
}

// isAuthError returns true for HTTP status codes that indicate
// authentication or authorization issues rather than broken links.
func isAuthError(statusCode int) bool {
	switch statusCode {
	case 401: // Unauthorized - missing or invalid authentication
	case 403: // Forbidden - authenticated but not authorized
	case 405: // Method Not Allowed - often due to missing authentication
		return true
	}
	return false
}

// handleBrokenLink creates and publishes a broken link event.
func (s *VerificationService) handleBrokenLink(ctx context.Context, absoluteURL string, link *Link, page *PageMetadata, status int, errorMsg string, cache *CacheEntry) {
	frontMatter := page.FrontMatter
	if frontMatter == nil && page.HugoPath != "" {
		if contentBytes, err := os.ReadFile(page.HugoPath); err == nil {
			if fm, err := ParseFrontMatter(contentBytes); err == nil {
				frontMatter = fm
			}
		}
	}

	event := &BrokenLinkEvent{
		URL:        absoluteURL,
		Status:     status,
		Error:      errorMsg,
		IsInternal: link.IsInternal,

		// Source metadata
		SourcePath:         page.DocFile.Path,
		SourceRelativePath: page.DocFile.RelativePath,
		Repository:         page.DocFile.Repository,
		Forge:              page.DocFile.Forge,
		Section:            page.DocFile.Section,
		FileName:           page.DocFile.Name,
		DocsBase:           page.DocFile.DocsBase,

		// Generated paths
		HugoPath:     page.HugoPath,
		RenderedPath: page.RenderedPath,
		RenderedURL:  page.RenderedURL,

		// Front matter (lazy-loaded to avoid per-page up-front parsing)
		FrontMatter: frontMatter,

		// Build context
		BuildID:   page.BuildID,
		BuildTime: page.BuildTime,
	}

	// Extract specific front matter fields
	if frontMatter != nil {
		if title, ok := frontMatter["title"].(string); ok {
			event.Title = title
		}
		if desc, ok := frontMatter["description"].(string); ok {
			event.Description = desc
		}
		if date, ok := frontMatter["date"].(string); ok {
			event.Date = date
		}
		if typ, ok := frontMatter["type"].(string); ok {
			event.Type = typ
		}
	}

	// Failure tracking from cache
	if cache != nil {
		event.FailureCount = cache.FailureCount
		event.FirstFailedAt = cache.FirstFailedAt
		event.LastChecked = cache.LastChecked
	}

	// Publish event
	if err := s.cache.PublishBrokenLink(ctx, event); err != nil {
		slog.Error("Failed to publish broken link event",
			"url", absoluteURL,
			"source", page.RenderedPath,
			"error", err)
	} else {
		slog.Warn("Broken link detected",
			"url", absoluteURL,
			"source", page.RenderedPath,
			"repository", page.DocFile.Repository,
			"status", status,
			"error", errorMsg)
	}
}

// ParseFrontMatter extracts front matter from transformed content.
// Returns ErrNoFrontMatter if content has no front matter.
func ParseFrontMatter(content []byte) (map[string]any, error) {
	if !hasFrontMatter(content) {
		return nil, ErrNoFrontMatter
	}

	// Extract front matter between --- delimiters
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) < 3 {
		return nil, ErrNoFrontMatter
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	return fm, nil
}

// hasFrontMatter checks if content has front matter.
func hasFrontMatter(content []byte) bool {
	return len(content) > 4 && string(content[0:3]) == "---"
}

// Close closes the verification service and releases resources.
func (s *VerificationService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil {
		return s.cache.Close()
	}

	return nil
}
