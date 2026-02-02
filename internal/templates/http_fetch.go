package templates

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxTemplateResponseBytes is the maximum size of template page responses (5MB).
// This prevents memory exhaustion from malicious or malformed responses.
const maxTemplateResponseBytes = 5 * 1024 * 1024

// NewTemplateHTTPClient creates an HTTP client configured for safe template fetching.
//
// The client has:
//   - 10 second timeout to prevent hanging requests
//   - Redirect protection (blocks cross-host redirects, limits to 5 redirects)
//   - No automatic cookie handling (stateless requests)
//
// Returns a client suitable for fetching template discovery pages and template pages.
func NewTemplateHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			if req.URL.Host != via[0].URL.Host {
				return errors.New("redirect to different host blocked")
			}
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

// FetchTemplateDiscovery fetches and parses the template discovery page from a documentation site.
//
// The discovery page is expected at <baseURL>/categories/templates/ and contains
// links to individual template pages.
//
// Parameters:
//   - ctx: Context for request cancellation/timeout
//   - baseURL: Base URL of the documentation site (e.g., "https://docs.example.com")
//   - client: HTTP client (if nil, uses NewTemplateHTTPClient())
//
// Returns:
//   - A slice of discovered TemplateLink structs
//   - An error if the URL is invalid, request fails, or parsing fails
//
// Example:
//
//	client := NewTemplateHTTPClient()
//	links, err := FetchTemplateDiscovery(ctx, "https://docs.example.com", client)
func FetchTemplateDiscovery(ctx context.Context, baseURL string, client *http.Client) ([]TemplateLink, error) {
	if client == nil {
		client = NewTemplateHTTPClient()
	}
	root, err := validateTemplateURL(baseURL)
	if err != nil {
		return nil, err
	}

	discoveryURL := *root
	discoveryURL.Path = strings.TrimSuffix(discoveryURL.Path, "/") + "/categories/templates/"

	body, err := fetchHTML(ctx, discoveryURL.String(), client)
	if err != nil {
		return nil, err
	}

	return ParseTemplateDiscovery(strings.NewReader(string(body)), root.String())
}

// FetchTemplatePage fetches and parses a single template page from a documentation site.
//
// The template page is an HTML document containing:
//   - Metadata in <meta property="docbuilder:template.*"> tags
//   - Template body in a <pre><code class="language-markdown"> block
//
// Parameters:
//   - ctx: Context for request cancellation/timeout
//   - templateURL: Full URL to the template page
//   - client: HTTP client (if nil, uses NewTemplateHTTPClient())
//
// Returns:
//   - A parsed TemplatePage with metadata and body
//   - An error if the URL is invalid, request fails, or parsing fails
//
// Example:
//
//	client := NewTemplateHTTPClient()
//	page, err := FetchTemplatePage(ctx, "https://docs.example.com/templates/adr.template/", client)
func FetchTemplatePage(ctx context.Context, templateURL string, client *http.Client) (*TemplatePage, error) {
	if client == nil {
		client = NewTemplateHTTPClient()
	}
	_, err := validateTemplateURL(templateURL)
	if err != nil {
		return nil, err
	}

	body, err := fetchHTML(ctx, templateURL, client)
	if err != nil {
		return nil, err
	}
	return ParseTemplatePage(strings.NewReader(string(body)))
}

// fetchHTML fetches HTML content from a URL with size limits and error handling.
//
// The function:
//   - Limits response size to maxTemplateResponseBytes
//   - Validates HTTP status codes (200-299)
//   - Handles request cancellation via context
func fetchHTML(ctx context.Context, pageURL string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", pageURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", pageURL, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxTemplateResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxTemplateResponseBytes {
		return nil, errors.New("response too large")
	}
	return data, nil
}

// validateTemplateURL validates that a URL is safe for template fetching.
//
// Only http:// and https:// schemes are allowed. This prevents file://, data:,
// and other potentially dangerous schemes.
func validateTemplateURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	return parsed, nil
}
