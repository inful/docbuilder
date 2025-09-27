package hugo

import (
	"fmt"
	"net/url"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// generatePerPageEditURL returns a theme-specific edit URL for a doc file or empty string if it should not be generated.
// Conditions for generation (current behavior parity):
//   - Theme is "hextra"
//   - Site-level params.editURL.base not set
//   - Corresponding repository config is found
//   - Repository URL matches a supported forge pattern (github, gitlab, bitbucket, gitea/forgejo)
// generatePerPageEditURL is retained for backward compatibility with existing callers.
// It now delegates to the shared EditLinkResolver. Prefer using Generator.editLinkResolver or BuildFrontMatter logic directly.
func generatePerPageEditURL(cfg *config.Config, file docs.DocFile) string {
    resolver := NewEditLinkResolver(cfg)
    return resolver.Resolve(file)
}

// resolveForgeForRepository attempts to match a repository clone URL against configured forge base URLs
// returning the forge type and canonical base URL. Returns empty strings if not resolvable.
func resolveForgeForRepository(cfg *config.Config, repoURL string) (config.ForgeType, string) {
	if cfg == nil || len(cfg.Forges) == 0 || repoURL == "" {
		return "", ""
	}
	// Normalize repoURL for comparison (strip ssh prefixes to https where possible)
	normalized := repoURL
	if strings.HasPrefix(normalized, "git@") {
		// Convert git@host:org/repo to https://host/org/repo for matching
		parts := strings.SplitN(strings.TrimPrefix(normalized, "git@"), ":", 2)
		if len(parts) == 2 {
			normalized = fmt.Sprintf("https://%s/%s", parts[0], parts[1])
		}
	}
	for _, fc := range cfg.Forges {
		if fc == nil || fc.BaseURL == "" { // need a baseURL for matching
			continue
		}
		base := strings.TrimSuffix(fc.BaseURL, "/")
		// Match if normalized starts with base or contains base host
		if strings.HasPrefix(normalized, base+"/") || strings.HasPrefix(normalized, base) {
			return fc.Type, base
		}
		// Also allow matching by host if user provided variant (e.g., http vs https)
		if u1, err1 := url.Parse(base); err1 == nil {
			if u2, err2 := url.Parse(normalized); err2 == nil && u1.Host != "" && u1.Host == u2.Host {
				return fc.Type, base
			}
		}
	}
	return "", ""
}
