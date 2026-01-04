package forge

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// newHTTPClient30s returns a shared HTTP client with a 30s timeout.
// The client respects HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables.
func newHTTPClient30s() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment, // Respects HTTP_PROXY, HTTPS_PROXY, NO_PROXY
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// withDefaults applies default API/base URLs when empty.
func withDefaults(apiURL, baseURL, defAPI, defBase string) (string, string) {
	if apiURL == "" {
		apiURL = defAPI
	}
	if baseURL == "" {
		baseURL = defBase
	}
	return apiURL, baseURL
}

// tokenFromConfig extracts token from forge config or returns an error.
func tokenFromConfig(fg *Config, forgeName string) (string, error) {
	if fg != nil && fg.Auth != nil && fg.Auth.Type == cfg.AuthTypeToken {
		return fg.Auth.Token, nil
	}
	return "", fmt.Errorf("%s client requires token authentication", forgeName)
}

// fetchAndConvertReposGeneric is a generic helper to reduce duplication between
// GitHub, GitLab, and Forgejo fetchAndConvertRepos functions.
// It fetches forge-specific repository types and converts them to common Repository format.
//
// Type parameter T represents the forge-specific repository type (e.g., githubRepo, forgejoRepo).
// The converter function transforms a forge-specific repo to the common *Repository format.
func fetchAndConvertReposGeneric[T any](
	base *BaseForge,
	ctx context.Context,
	endpoint string,
	pageParam, sizeParam string,
	pageSize int,
	converter func(*T) *Repository,
) ([]*Repository, error) {
	// Fetch forge-specific repositories using pagination
	forgeRepos, err := PaginatedFetchHelper(
		ctx,
		endpoint,
		pageParam,
		sizeParam,
		pageSize,
		func(ep string) ([]T, bool, error) {
			req, reqErr := base.NewRequest(ctx, "GET", ep, nil)
			if reqErr != nil {
				return nil, false, reqErr
			}

			var repos []T
			if doErr := base.DoRequest(req, &repos); doErr != nil {
				return nil, false, doErr
			}

			// Has more if we got a full page
			hasMore := len(repos) >= pageSize
			return repos, hasMore, nil
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to common Repository format
	allRepos := make([]*Repository, 0, len(forgeRepos))
	for i := range forgeRepos {
		repo := converter(&forgeRepos[i])
		allRepos = append(allRepos, repo)
	}

	return allRepos, nil
}
