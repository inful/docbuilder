package hugo

import (
	"fmt"
	"net/url"
	"path/filepath"
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
func generatePerPageEditURL(cfg *config.Config, file docs.DocFile) string {
	if cfg == nil || cfg.Hugo.ThemeType() != config.ThemeHextra {
		return ""
	}
	if cfg.Hugo.Params != nil { // site-level base suppression
		if v, ok := cfg.Hugo.Params["editURL"]; ok {
			if m, ok := v.(map[string]any); ok {
				if b, ok := m["base"].(string); ok && b != "" {
					return ""
				}
			}
		}
	}
	// Find repo config
	var repoCfg *config.Repository
	for i := range cfg.Repositories {
		if cfg.Repositories[i].Name == file.Repository {
			repoCfg = &cfg.Repositories[i]
			break
		}
	}
	if repoCfg == nil {
		return ""
	}
	branch := repoCfg.Branch
	if branch == "" {
		branch = "main"
	}
	repoRel := file.RelativePath
	if base := strings.TrimSpace(file.DocsBase); base != "" && base != "." {
		repoRel = filepath.ToSlash(filepath.Join(base, repoRel))
	} else {
		repoRel = filepath.ToSlash(repoRel)
	}
	// Normalize clone URL (strip .git, convert SSH to https for known providers)
	raw := strings.TrimSuffix(repoCfg.URL, ".git")

	// Resolve forge type & base URL from configured forges first (authoritative)
	forgeType, forgeBase := resolveForgeForRepository(cfg, raw)

	// Fallback legacy heuristics if forge not resolved (keeps backward compat for bitbucket or unconfigured public forges)
	if forgeType == "" {
		switch {
		case strings.Contains(raw, "github.com"):
			forgeType = config.ForgeGitHub
			forgeBase = "https://github.com"
		case strings.Contains(raw, "gitlab.com"):
			forgeType = config.ForgeGitLab
			forgeBase = "https://gitlab.com"
		case strings.Contains(raw, "bitbucket.org"):
			// Bitbucket not (yet) a supported ForgeType; preserve existing behavior
			return fmt.Sprintf("%s/src/%s/%s?mode=edit", raw, branch, repoRel)
		}
	}

	// SSH to https translation for github / gitlab when not matched via config
	if forgeType == config.ForgeGitHub && strings.HasPrefix(raw, "git@github.com:") {
		raw = "https://github.com/" + strings.TrimPrefix(raw, "git@github.com:")
	} else if forgeType == config.ForgeGitLab && strings.HasPrefix(raw, "git@gitlab.com:") {
		raw = "https://gitlab.com/" + strings.TrimPrefix(raw, "git@gitlab.com:")
	}

	// If we have an authoritative forgeBase, attempt to rewrite raw to use it exactly
	if forgeBase != "" {
		if u, err := url.Parse(raw); err == nil {
			// Expect path like /org/repo or org/repo (ssh converted)
			path := strings.TrimPrefix(u.Path, "/")
			// Some raw may already include host/path (https); unify
			raw = fmt.Sprintf("%s/%s", strings.TrimSuffix(forgeBase, "/"), path)
		}
	}

	switch forgeType {
	case config.ForgeGitHub:
		return fmt.Sprintf("%s/edit/%s/%s", raw, branch, repoRel)
	case config.ForgeGitLab:
		return fmt.Sprintf("%s/-/edit/%s/%s", raw, branch, repoRel)
	case config.ForgeForgejo:
		return fmt.Sprintf("%s/_edit/%s/%s", raw, branch, repoRel)
	default:
		return ""
	}
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
