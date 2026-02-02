package commands

import (
	"fmt"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

const templateBaseURLEnv = "DOCBUILDER_TEMPLATE_BASE_URL"

// ResolveTemplateBaseURL resolves the template base URL based on flags, env, and config.
func ResolveTemplateBaseURL(flagBaseURL string, cfg *config.Config) (string, error) {
	if flagBaseURL != "" {
		return flagBaseURL, nil
	}
	if env := os.Getenv(templateBaseURLEnv); env != "" {
		return env, nil
	}
	if cfg != nil && cfg.Hugo.BaseURL != "" {
		return cfg.Hugo.BaseURL, nil
	}
	return "", fmt.Errorf("template base URL is required (set --base-url, %s, or hugo.base_url)", templateBaseURLEnv)
}
