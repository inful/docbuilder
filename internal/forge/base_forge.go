package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// BaseForge provides common HTTP operations for forge clients.
// It consolidates duplicate newRequest/doRequest logic from GitHub, GitLab, and Forgejo clients.
type BaseForge struct {
	httpClient *http.Client
	apiURL     string
	token      string

	// Forge-specific customization hooks
	authHeaderPrefix string // "Bearer " for GitHub/GitLab, "token " for Forgejo
	customHeaders    map[string]string
}

// NewBaseForge creates a BaseForge with common forge HTTP client settings.
func NewBaseForge(httpClient *http.Client, apiURL, token string) *BaseForge {
	return &BaseForge{
		httpClient:       httpClient,
		apiURL:           apiURL,
		token:            token,
		authHeaderPrefix: "Bearer ", // default to Bearer
		customHeaders:    make(map[string]string),
	}
}

// SetAuthHeaderPrefix customizes the authorization header format (e.g., "token " for Forgejo).
func (b *BaseForge) SetAuthHeaderPrefix(prefix string) {
	b.authHeaderPrefix = prefix
}

// SetCustomHeader sets forge-specific headers (e.g., GitHub API version, GitLab-specific headers).
func (b *BaseForge) SetCustomHeader(key, value string) {
	b.customHeaders[key] = value
}

// NewRequest creates an HTTP request with common forge patterns.
// Handles URL building, body encoding, and header setting.
// Endpoint should be relative path like "/user/orgs" or "repos/{owner}/{repo}".
// For Forgejo compatibility, query strings in endpoint are properly handled.
func (b *BaseForge) NewRequest(ctx context.Context, method, endpoint string, body any) (*http.Request, error) {
	// Parse endpoint to handle query strings and leading slashes
	cleanEndpoint := strings.TrimPrefix(endpoint, "/")

	var rawQuery string
	if idx := strings.Index(cleanEndpoint, "?"); idx != -1 {
		rawQuery = cleanEndpoint[idx+1:]
		cleanEndpoint = cleanEndpoint[:idx]
	}

	u, err := url.Parse(b.apiURL)
	if err != nil {
		return nil, errors.ForgeError("failed to parse API URL").
			WithCause(err).
			WithContext("api_url", b.apiURL).
			Build()
	}

	// Join paths while preserving base path
	basePath := strings.TrimSuffix(u.Path, "/")
	u.Path = path.Join(basePath, cleanEndpoint)
	if rawQuery != "" {
		u.RawQuery = rawQuery
	}

	// Build request with optional body
	var req *http.Request
	if body != nil {
		var jsonBody []byte
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, errors.ForgeError("failed to marshal request body").
				WithCause(err).
				Build()
		}
		req, err = http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(jsonBody))
		if err != nil {
			return nil, errors.ForgeError("failed to create request").
				WithCause(err).
				WithContext("method", method).
				WithContext("url", u.String()).
				Build()
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, u.String(), http.NoBody)
		if err != nil {
			return nil, errors.ForgeError("failed to create request").
				WithCause(err).
				WithContext("method", method).
				WithContext("url", u.String()).
				Build()
		}
	}

	// Set common headers
	req.Header.Set("Authorization", b.authHeaderPrefix+b.token)
	req.Header.Set("User-Agent", "DocBuilder/1.0")

	// Apply forge-specific custom headers
	for key, value := range b.customHeaders {
		req.Header.Set(key, value)
	}

	return req, nil
}

// DoRequest executes an HTTP request and decodes the response.
// Consolidates error handling, response closing, and JSON decoding.
func (b *BaseForge) DoRequest(req *http.Request, result any) error {
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return errors.NetworkError("failed to execute forge request").
			WithCause(err).
			WithContext("method", req.Method).
			WithContext("url", req.URL.String()).
			Build()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		// Read limited body for diagnostics
		limitedBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		bodyStr := strings.ReplaceAll(string(limitedBody), "\n", " ")

		category := errors.CategoryForge
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			category = errors.CategoryAuth
		case http.StatusNotFound:
			category = errors.CategoryNotFound
		}

		return errors.NewError(category, fmt.Sprintf("forge API error: %s", resp.Status)).
			WithContext("status", resp.Status).
			WithContext("code", resp.StatusCode).
			WithContext("url", req.URL.String()).
			WithContext("response", bodyStr).
			Build()
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return errors.ForgeError("failed to decode response").
				WithCause(err).
				Build()
		}
	}

	return nil
}

// DoRequestWithHeaders is like DoRequest but also returns response headers.
// Useful for pagination that uses Link headers (GitHub).
func (b *BaseForge) DoRequestWithHeaders(req *http.Request, result any) (http.Header, error) {
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, errors.NetworkError("failed to execute forge request").
			WithCause(err).
			WithContext("method", req.Method).
			WithContext("url", req.URL.String()).
			Build()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		limitedBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		bodyStr := strings.ReplaceAll(string(limitedBody), "\n", " ")

		category := errors.CategoryForge
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			category = errors.CategoryAuth
		case http.StatusNotFound:
			category = errors.CategoryNotFound
		}

		return nil, errors.NewError(category, "forge API error").
			WithContext("status", resp.Status).
			WithContext("code", resp.StatusCode).
			WithContext("url", req.URL.String()).
			WithContext("response", bodyStr).
			Build()
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return nil, errors.ForgeError("failed to decode response").
				WithCause(err).
				Build()
		}
	}

	return resp.Header, nil
}

// PaginatedFetchHelper performs paginated API requests using a common pattern.
// The caller provides:
// - ctx: context for cancellation
// - baseEndpoint: API path without pagination params (e.g., "/orgs/myorg/repos")
// - pageParam: name of page parameter ("page" for most forges)
// - limitParam: name of limit/per_page parameter
// - pageSize: items per page
// - fetchPage: callback that receives full endpoint and returns items + hasMore + error
//
// Returns all accumulated results or an error.
func PaginatedFetchHelper[T any](
	ctx context.Context,
	baseEndpoint string,
	pageParam string,
	limitParam string,
	pageSize int,
	fetchPage func(endpoint string) ([]T, bool, error),
) ([]T, error) {
	var allResults []T
	page := 1

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Build paginated endpoint
		sep := "?"
		if strings.Contains(baseEndpoint, "?") {
			sep = "&"
		}
		endpoint := fmt.Sprintf("%s%s%s=%d&%s=%d", baseEndpoint, sep, pageParam, page, limitParam, pageSize)

		// Fetch this page
		pageResults, hasMore, err := fetchPage(endpoint)
		if err != nil {
			return nil, err
		}

		allResults = append(allResults, pageResults...)

		// Stop if no more pages
		if !hasMore || len(pageResults) < pageSize {
			break
		}

		page++
	}

	return allResults, nil
}
