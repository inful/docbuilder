package linkverify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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
	// ContentPath is the source markdown of the page on disk. The front
	// matter is read from it only when a broken link needs it, so a build
	// does not read every content file again.
	ContentPath string
	BaseURL     string    // Site base URL
	BuildID     string    // Build identifier
	BuildTime   time.Time // Build timestamp
	ContentHash string    // MD5 hash of page content for change detection

	// Links are the links extracted from the rendered page. The caller
	// that already read the page fills them in, so the page does not
	// have to be read a second time.
	Links []*Link
}

// frontMatter returns the front matter of the page. It is read from the
// source file on demand: only a broken link needs it, so most builds never
// read the content files again.
func (p *PageMetadata) frontMatter() map[string]any {
	if p.FrontMatter != nil {
		return p.FrontMatter
	}
	if p.ContentPath == "" {
		return nil
	}
	contentBytes, err := os.ReadFile(filepath.Clean(p.ContentPath))
	if err != nil {
		return nil
	}
	fm, err := ParseFrontMatter(contentBytes)
	if err != nil {
		return nil
	}
	p.FrontMatter = fm
	return fm
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
	rateDelay  time.Duration // Delay between link verification requests
}

// maxConcurrent returns how many links are verified at the same time.
// A configuration that asks for none, or for a negative number, would
// stop the daemon: a channel of a negative size is a panic.
func maxConcurrent(configured int) int {
	if configured < 1 {
		return 1
	}
	return configured
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

	// Parse rate limit delay
	rateDelay, err := time.ParseDuration(cfg.RateLimitDelay)
	if err != nil {
		rateDelay = 100 * time.Millisecond
	}

	// Create HTTP transport with proxy support
	// This respects HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Create HTTP client with timeout and proxy support
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if cfg.FollowRedirects != nil && !*cfg.FollowRedirects {
				return http.ErrUseLastResponse
			}
			// len(via) is the number of previous requests already made.
			// If MaxRedirects is 3, we should allow 3 redirects (i.e., 4 total requests).
			if len(via) > cfg.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
			}
			return nil
		},
	}

	return &VerificationService{
		cfg:        cfg,
		nats:       natsClient,
		httpClient: httpClient,
		semaphore:  make(chan struct{}, maxConcurrent(cfg.MaxConcurrent)),
		rateDelay:  rateDelay,
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

	// The pages are verified by a fixed number of workers and not by
	// one goroutine per page: a portal with thousands of pages spawned
	// thousands of goroutines, and every one of them held the links of
	// its page in memory while it waited for its turn.
	workers := min(maxConcurrent(s.cfg.MaxConcurrent), max(len(pages), 1))

	queue := make(chan *PageMetadata)
	linkQueue := make(chan linkJob)
	for range workers {
		s.wg.Go(func() {
			for page := range queue {
				s.verifyPage(ctx, page, linkQueue)
			}
		})
		s.wg.Go(func() {
			for job := range linkQueue {
				s.verifyLink(ctx, job.link, job.page)
			}
		})
	}

	for _, page := range pages {
		select {
		case <-ctx.Done():
			slog.Info("Link verification canceled")
			close(queue)
			close(linkQueue)
			s.wg.Wait()
			return ctx.Err()
		case queue <- page:
		}
	}
	close(queue)
	close(linkQueue)

	// Wait for all verifications to complete
	s.wg.Wait()
	slog.Info("Link verification completed")

	return nil
}

// verifyPage verifies all links in a single page.
func (s *VerificationService) verifyPage(ctx context.Context, page *PageMetadata, linkQueue chan linkJob) {
	// Check if page content has changed using MD5 hash
	if page.ContentHash != "" {
		if cachedHash, err := s.nats.GetPageHash(ctx, page.RenderedPath); err == nil && cachedHash == page.ContentHash {
			slog.Debug("Skipping link verification for unchanged page",
				"path", page.RenderedPath,
				"hash", page.ContentHash[:8])
			return
		}
	}

	// Extract links from HTML. The caller may have extracted them already
	// while reading the page for its content hash - do not read it twice.
	links := page.Links
	if links == nil {
		var err error
		links, err = ExtractLinks(page.HTMLPath, page.BaseURL)
		if err != nil {
			slog.Warn("Failed to extract links from page",
				"path", page.HTMLPath,
				"error", err)
			return
		}
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

		// The link is verified by the workers of the pool. One
		// goroutine per link spawned tens of thousands of them on a
		// portal with thousands of pages.
		linkQueue <- linkJob{link: link, page: page}
	}

	// All links verified for this page - update page hash in cache
	if page.ContentHash != "" {
		if err := s.nats.SetPageHash(ctx, page.RenderedPath, page.ContentHash); err != nil {
			slog.Debug("Failed to cache page hash", "path", page.RenderedPath, "error", err)
		}
	}
}

// linkJob is a link to verify and the page it belongs to.
type linkJob struct {
	link *Link
	page *PageMetadata
}

// verifyLink verifies a single link.
func (s *VerificationService) verifyLink(ctx context.Context, link *Link, page *PageMetadata) {
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

	// Rate limiting: the delay belongs in front of the request, not in
	// front of the page, so it throttles the requests as documented.
	if s.rateDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.rateDelay):
		}
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
	if isTimeoutError(err) {
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
		if isTimeoutError(errGet) {
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

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	nerr, ok := errors.AsType[net.Error](err)
	if !ok {
		return false
	}
	return nerr.Timeout()
}

// docField reads a field of the page's document file, or the empty string
// when the page has no document file.
func docField(page *PageMetadata, field func(*docs.DocFile) string) string {
	if page == nil || page.DocFile == nil {
		return ""
	}
	return field(page.DocFile)
}

// handleBrokenLink creates and publishes a broken link event.
func (s *VerificationService) handleBrokenLink(ctx context.Context, absoluteURL string, link *Link, page *PageMetadata, status int, errorMsg string, cache *CacheEntry) {
	event := &BrokenLinkEvent{
		URL:        absoluteURL,
		Status:     status,
		Error:      errorMsg,
		IsInternal: link.IsInternal,

		// Source metadata. The document file is optional: a caller that
		// only has the rendered page does not have one, and a nil pointer
		// here would take the daemon down.
		SourcePath:         docField(page, func(d *docs.DocFile) string { return d.Path }),
		SourceRelativePath: docField(page, func(d *docs.DocFile) string { return d.RelativePath }),
		Repository:         docField(page, func(d *docs.DocFile) string { return d.Repository }),
		Forge:              docField(page, func(d *docs.DocFile) string { return d.Forge }),
		Section:            docField(page, func(d *docs.DocFile) string { return d.Section }),
		FileName:           docField(page, func(d *docs.DocFile) string { return d.Name }),
		DocsBase:           docField(page, func(d *docs.DocFile) string { return d.DocsBase }),

		// Generated paths
		HugoPath:     page.HugoPath,
		RenderedPath: page.RenderedPath,
		RenderedURL:  page.RenderedURL,

		// Front matter
		FrontMatter: page.frontMatter(),

		// Build context
		BuildID:   page.BuildID,
		BuildTime: page.BuildTime,
	}

	// Extract specific front matter fields
	if fm := page.frontMatter(); fm != nil {
		if title, ok := fm["title"].(string); ok {
			event.Title = title
		}
		if desc, ok := fm["description"].(string); ok {
			event.Description = desc
		}
		if date, ok := fm["date"].(string); ok {
			event.Date = date
		}
		if typ, ok := fm["type"].(string); ok {
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
			"repository", docField(page, func(d *docs.DocFile) string { return d.Repository }),
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
