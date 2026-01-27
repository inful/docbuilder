package linkverify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
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
		return nil, errors.New("link verification is disabled")
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

	for _, page := range pages {
		select {
		case <-ctx.Done():
			slog.Info("Link verification canceled")
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

	// Check if page content has changed using MD5 hash
	if page.ContentHash != "" {
		if cachedHash, err := s.nats.GetPageHash(ctx, page.RenderedPath); err == nil && cachedHash == page.ContentHash {
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
	for _, link := range links {
		select {
		case <-ctx.Done():
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

		// Acquire semaphore
		s.semaphore <- struct{}{}
		s.wg.Add(1)
		go s.verifyLink(ctx, link, page)
	}

	// All links verified for this page - update page hash in cache
	if page.ContentHash != "" {
		if err := s.nats.SetPageHash(ctx, page.RenderedPath, page.ContentHash); err != nil {
			slog.Debug("Failed to cache page hash", "path", page.RenderedPath, "error", err)
		}
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
	cached, err := s.nats.GetCachedResult(ctx, absoluteURL)
	if err != nil {
		slog.Debug("Cache lookup error", "url", absoluteURL, "error", err)
	} else if cached != nil && s.nats.IsCacheValid(cached) {
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
	if err := s.nats.SetCachedResult(ctx, cacheEntry); err != nil {
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
	baseURL := ""
	if page != nil {
		baseURL = page.BaseURL
	}
	slog.Debug("Checking link",
		"url", linkURL,
		"is_internal", isInternal,
		"base_url", baseURL)

	// Internal links should be verified against the rendered site on disk.
	// This avoids false positives when the public base URL is temporarily unavailable
	// or does not support HEAD.
	if page != nil {
		if isInternal || urlHostMatchesBase(linkURL, page.BaseURL) {
			return checkInternalLink(page, linkURL)
		}
	}

	return s.checkExternalLink(ctx, linkURL)
}

func urlHostMatchesBase(linkURL, baseURL string) bool {
	u, err := url.Parse(linkURL)
	if err != nil {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	if u.Hostname() == "" || base.Hostname() == "" {
		return false
	}
	return strings.EqualFold(u.Hostname(), base.Hostname())
}

func checkInternalLink(page *PageMetadata, absoluteURL string) (int, error) {
	localPath, err := localPathForInternalURL(page, absoluteURL)
	if err != nil {
		return 0, err
	}
	st, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, fmt.Errorf("internal file not found: %s", localPath)
		}
		return 0, fmt.Errorf("failed to stat internal file: %w", err)
	}
	if st.IsDir() {
		return http.StatusNotFound, fmt.Errorf("internal path is a directory: %s", localPath)
	}
	return http.StatusOK, nil
}

func localPathForInternalURL(page *PageMetadata, absoluteURL string) (string, error) {
	publicDir, err := publicDirForPage(page)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(absoluteURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	urlPath := u.EscapedPath()
	if urlPath == "" {
		urlPath = "/"
	}
	urlPath = path.Clean(urlPath)
	if urlPath == "." {
		urlPath = "/"
	}

	// Hugo sites commonly use "pretty URLs" where "/section/" maps to "/section/index.html".
	switch {
	case urlPath == "/":
		urlPath = "/index.html"
	case strings.HasSuffix(u.Path, "/"):
		urlPath = strings.TrimSuffix(urlPath, "/") + "/index.html"
	case path.Ext(urlPath) == "":
		urlPath += "/index.html"
	}

	rel := strings.TrimPrefix(urlPath, "/")
	return filepath.Join(publicDir, filepath.FromSlash(rel)), nil
}

func publicDirForPage(page *PageMetadata) (string, error) {
	if page == nil {
		return "", errors.New("page metadata is required")
	}
	if page.HTMLPath == "" || page.RenderedPath == "" {
		return "", errors.New("page HTMLPath and RenderedPath are required")
	}

	// HTMLPath == <publicDir>/<RenderedPath>
	// Derive <publicDir> by walking up from HTMLPath based on RenderedPath depth.
	htmlDir := filepath.Dir(page.HTMLPath)
	relDir := filepath.Dir(page.RenderedPath)
	if relDir == "." {
		return htmlDir, nil
	}

	// Count path segments in relDir and walk up.
	segments := strings.Split(filepath.ToSlash(relDir), "/")
	publicDir := htmlDir
	for _, seg := range segments {
		if seg == "" || seg == "." {
			continue
		}
		publicDir = filepath.Dir(publicDir)
	}
	return publicDir, nil
}

// checkExternalLink verifies an external link via HTTP request.
func (s *VerificationService) checkExternalLink(ctx context.Context, linkURL string) (int, error) {
	// First try HEAD (cheap, but some sites return false negatives for HEAD).
	status, err := s.doExternalRequest(ctx, http.MethodHead, linkURL)
	if err == nil {
		return status, nil
	}

	// Treat rate limiting as "not broken". These responses indicate the URL likely exists,
	// but the remote site is asking us to slow down.
	if isRateLimited(status) {
		return status, nil
	}

	// If HEAD is rejected or unhelpful, retry with a lightweight GET.
	// Common cases:
	// - Some CDNs/WAFs return 404 for HEAD but 200 for GET
	// - Some servers mishandle HEAD on dynamic routes
	switch status {
	case http.StatusNotFound, http.StatusBadRequest:
		statusGet, errGet := s.doExternalRequest(ctx, http.MethodGet, linkURL)
		if errGet == nil {
			return statusGet, nil
		}
		if isRateLimited(statusGet) {
			return statusGet, nil
		}
		return statusGet, errGet
	default:
		return status, err
	}
}

func (s *VerificationService) doExternalRequest(ctx context.Context, method, linkURL string) (int, error) {
	// URL fragments are not sent to servers; strip them to avoid confusing redirects/logging.
	if u, parseErr := url.Parse(linkURL); parseErr == nil {
		u.Fragment = ""
		linkURL = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, method, linkURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Headers that improve compatibility with WAF/CDN protected sites.
	req.Header.Set("User-Agent", "DocBuilder-LinkVerifier/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if method == http.MethodGet {
		// Keep GETs lightweight when possible.
		req.Header.Set("Range", "bytes=0-1023")
	}

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

func isRateLimited(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests
}

// handleBrokenLink creates and publishes a broken link event.
func (s *VerificationService) handleBrokenLink(ctx context.Context, absoluteURL string, link *Link, page *PageMetadata, status int, errorMsg string, cache *CacheEntry) {
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
	if err := s.nats.PublishBrokenLink(ctx, event); err != nil {
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
	doc, err := docmodel.Parse(content, docmodel.Options{})
	if err != nil {
		// Preserve legacy behavior: a frontmatter split/parse failure is treated as
		// "no front matter" for link verification metadata.
		return nil, ErrNoFrontMatter
	}
	if !doc.HadFrontmatter() {
		return nil, ErrNoFrontMatter
	}
	if len(bytes.TrimSpace(doc.FrontmatterRaw())) == 0 {
		return map[string]any{}, nil
	}

	fm, err := doc.FrontmatterFields()
	if err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}
	return fm, nil
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
