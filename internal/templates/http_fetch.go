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

const maxTemplateResponseBytes = 5 * 1024 * 1024

// NewTemplateHTTPClient creates an HTTP client with safe defaults.
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

// FetchTemplateDiscovery retrieves and parses the template discovery page.
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

	return ParseTemplateDiscovery(root.String(), strings.NewReader(string(body)))
}

// FetchTemplatePage retrieves and parses a template page.
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
