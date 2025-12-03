package forge

import (
	"fmt"
	"net/http"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// newHTTPClient30s returns a shared HTTP client with a 30s timeout.
func newHTTPClient30s() *http.Client { //nolint:ireturn // returning concrete *http.Client is intentional; callers need full API
	return &http.Client{Timeout: 30 * time.Second}
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
