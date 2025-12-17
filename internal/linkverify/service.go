package linkverify

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"gopkg.in/yaml.v3"
)

// PageMetadata contains metadata about a page for verification.
type PageMetadata struct {
	DocFile      *docs.DocFile          // Original doc file reference
	HTMLPath     string                 // Path to rendered HTML file
	HugoPath     string                 // Hugo content path
	RenderedPath string                 // Path in rendered site
	RenderedURL  string                 // Full URL of rendered page
	FrontMatter  map[string]interface{} // Parsed front matter
	BaseURL      string                 // Site base URL
	BuildID      string                 // Build identifier
	BuildTime    time.Time              // Build timestamp
}

// VerificationService manages link verification operations.
type VerificationService struct {
	cfg        *config.LinkVerificationConfig
	nats       *NATSClient
	httpClient *http.Client
	mu         sync.Mutex
	running    bool
	wg         sync.WaitGroup
	semaphore  chan struct{} // Limit concurrent verifications
}

// NewVerificationService creates a new link verification service.
func NewVerificationService(cfg *config.LinkVerificationConfig) (*VerificationService, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("link verification is disabled")
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
		nats:       natsClient,
		httpClient: httpClient,
		semaphore:  make(chan struct{}, cfg.MaxConcurrent),
	}, nil
}

// VerifyPages verifies all links in the given pages.
func (s *VerificationService) VerifyPages(ctx context.Context, pages []*PageMetadata) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("verification already running")
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

	for _, page := range pages {
		select {
		case <-ctx.Done():
			slog.Info("Link verification cancelled")
			s.wg.Wait()
			return ctx.Err()
		default:
		}

		// Rate limiting
		time.Sleep(delay)

		// Verify page in background
		s.wg.Add(1)
		go s.verifyPage(ctx, page)
	}

	// Wait for all verifications to complete
	s.wg.Wait()
	slog.Info("Link verification completed")

	return nil
}

// verifyPage verifies all links in a single page.
func (s *VerificationService) verifyPage(ctx context.Context, page *PageMetadata) {
	defer s.wg.Done()

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
	for _, link := range links {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !ShouldVerifyLink(link) {
			continue
		}

		// Acquire semaphore
		s.semaphore <- struct{}{}
		s.wg.Add(1)
		go s.verifyLink(ctx, link, page)
	}
}

// verifyLink verifies a single link.
func (s *VerificationService) verifyLink(ctx context.Context, link *Link, page *PageMetadata) {
	defer s.wg.Done()
	defer func() { <-s.semaphore }()

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
	cached, err := s.nats.GetCachedResult(absoluteURL)
	if err != nil {
		slog.Debug("Cache lookup error", "url", absoluteURL, "error", err)
	} else if cached != nil && s.nats.IsCacheValid(cached) {
		// Use cached result
		if !cached.IsValid {
			// Still broken, update failure count and possibly publish
			s.handleBrokenLink(absoluteURL, link, page, cached.Status, cached.Error, cached)
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

		// Update failure tracking
		if cached != nil {
			cacheEntry.FailureCount = cached.FailureCount + 1
			cacheEntry.FirstFailedAt = cached.FirstFailedAt
			if cacheEntry.FirstFailedAt.IsZero() {
				cacheEntry.FirstFailedAt = time.Now()
			}
		} else {
			cacheEntry.FailureCount = 1
			cacheEntry.FirstFailedAt = time.Now()
		}
		cacheEntry.ConsecutiveFail = true

		// Handle broken link
		s.handleBrokenLink(absoluteURL, link, page, status, verifyErr.Error(), cacheEntry)
	} else {
		// Link is valid, reset failure count
		cacheEntry.FailureCount = 0
		cacheEntry.ConsecutiveFail = false
	}

	// Update cache
	if err := s.nats.SetCachedResult(cacheEntry); err != nil {
		slog.Warn("Failed to update cache", "url", absoluteURL, "error", err)
	}
}

// checkLink performs the actual link verification.
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

	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.StatusCode, nil
}

// handleBrokenLink creates and publishes a broken link event.
func (s *VerificationService) handleBrokenLink(absoluteURL string, link *Link, page *PageMetadata, status int, errorMsg string, cache *CacheEntry) {
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

		// Front matter
		FrontMatter: page.FrontMatter,

		// Build context
		BuildID:   page.BuildID,
		BuildTime: page.BuildTime,
	}

	// Extract specific front matter fields
	if page.FrontMatter != nil {
		if title, ok := page.FrontMatter["title"].(string); ok {
			event.Title = title
		}
		if desc, ok := page.FrontMatter["description"].(string); ok {
			event.Description = desc
		}
		if date, ok := page.FrontMatter["date"].(string); ok {
			event.Date = date
		}
		if typ, ok := page.FrontMatter["type"].(string); ok {
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
	if err := s.nats.PublishBrokenLink(event); err != nil {
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
func ParseFrontMatter(content []byte) (map[string]interface{}, error) {
	if !hasFrontMatter(content) {
		return nil, nil
	}

	// Extract front matter between --- delimiters
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) < 3 {
		return nil, nil
	}

	var fm map[string]interface{}
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

	if s.nats != nil {
		return s.nats.Close()
	}

	return nil
}
