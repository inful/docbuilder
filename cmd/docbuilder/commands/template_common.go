package commands

import (
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
	return "", derrors.NewError(derrors.CategoryConfig, "template base URL is required (set --base-url, "+templateBaseURLEnv+", or hugo.base_url)").Build()
}
