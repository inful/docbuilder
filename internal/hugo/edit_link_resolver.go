package hugo

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// EditLinkResolver encapsulates logic for deriving per-page edit links based on configuration and theme features.
// It is intentionally stateless aside from holding a reference to config; future enhancements (e.g. caching repo lookups)
// can be added without changing call sites.
type EditLinkResolver struct {
	cfg *config.Config
}

// NewEditLinkResolver creates a new resolver bound to the provided config (nil-safe).
func NewEditLinkResolver(cfg *config.Config) *EditLinkResolver {
	return &EditLinkResolver{cfg: cfg}
}

// Resolve returns the edit URL for the provided doc file or empty string if one should not be generated.
// Logic mirrors the previously inlined generatePerPageEditURL to preserve output parity.
func (r *EditLinkResolver) Resolve(file docs.DocFile) string {
	cfg := r.cfg
	if cfg == nil || cfg.Hugo.ThemeType() != config.ThemeHextra { // only Hextra currently supports per-page edit links
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
	cloneURL := strings.TrimSuffix(repoCfg.URL, ".git")

	// Gather tagged metadata if present.
	var forgeType config.ForgeType
	var fullName string
	if repoCfg.Tags != nil {
		if t, ok := repoCfg.Tags["forge_type"]; ok {
			forgeType = config.NormalizeForgeType(t)
		}
		if fn, ok := repoCfg.Tags["full_name"]; ok && fn != "" {
			fullName = fn
		}
	}

	// Resolve against configured forges to obtain canonical base_url (works for all supported forge types including self-hosted).
	resolvedType, resolvedBase := resolveForgeForRepository(cfg, cloneURL)
	if forgeType == "" {
		forgeType = resolvedType
	}

	// Derive full name if not provided.
	if fullName == "" {
		normalized := cloneURL
		if strings.HasPrefix(normalized, "git@") {
			parts := strings.SplitN(strings.TrimPrefix(normalized, "git@"), ":", 2)
			if len(parts) == 2 {
				normalized = fmt.Sprintf("https://%s/%s", parts[0], parts[1])
			}
		}
		if u, err := url.Parse(normalized); err == nil {
			fullName = strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
		}
	}

	// If still no forge type, attempt heuristic host mapping (includes Bitbucket special case)
	if forgeType == "" {
		switch {
		case strings.Contains(cloneURL, "github."):
			forgeType = config.ForgeGitHub
		case strings.Contains(cloneURL, "gitlab."):
			forgeType = config.ForgeGitLab
		case strings.Contains(cloneURL, "bitbucket.org"):
			if fullName != "" {
				return fmt.Sprintf("%s/src/%s/%s?mode=edit", cloneURL, branch, repoRel)
			}
			return ""
		case strings.Contains(cloneURL, "forgejo") || strings.Contains(cloneURL, "gitea"):
			forgeType = config.ForgeForgejo
		}
	}

	// Fallback: if still unknown but we have a fullname, treat as Forgejo/Gitea-style self-hosted instance
	// This preserves previous behavior where tests expected Gitea-style edit links without explicit forge config.
	if forgeType == "" && fullName != "" {
		forgeType = config.ForgeForgejo
	}

	if forgeType == "" || fullName == "" {
		return ""
	}

	// Determine base URL precedence: resolved forge base, else derive from cloneURL host, else public defaults.
	base := resolvedBase
	if base == "" {
		if u, err := url.Parse(cloneURL); err == nil && u.Scheme != "" && u.Host != "" {
			base = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
		if base == "" { // absolute last resort defaults (public SaaS)
			switch forgeType {
			case config.ForgeGitHub:
				base = "https://github.com"
			case config.ForgeGitLab:
				base = "https://gitlab.com"
			case config.ForgeForgejo:
				base = cloneURL // will be combined below
			}
		}
	}
	base = strings.TrimSuffix(base, "/")

	return forge.GenerateEditURL(forgeType, base, fullName, branch, repoRel)
}

// resolveForgeForRepository attempts to match a repository clone URL against configured forge base URLs
// returning the forge type and canonical base URL. Returns empty strings if not resolvable.
func resolveForgeForRepository(cfg *config.Config, repoURL string) (config.ForgeType, string) {
	if cfg == nil || len(cfg.Forges) == 0 || repoURL == "" {
		return "", ""
	}
	normalized := repoURL
	if strings.HasPrefix(normalized, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(normalized, "git@"), ":", 2)
		if len(parts) == 2 {
			normalized = fmt.Sprintf("https://%s/%s", parts[0], parts[1])
		}
	}
	for _, fc := range cfg.Forges {
		if fc == nil || fc.BaseURL == "" {
			continue
		}
		base := strings.TrimSuffix(fc.BaseURL, "/")
		if strings.HasPrefix(normalized, base+"/") || strings.HasPrefix(normalized, base) {
			return fc.Type, base
		}
		if u1, err1 := url.Parse(base); err1 == nil {
			if u2, err2 := url.Parse(normalized); err2 == nil && u1.Host != "" && u1.Host == u2.Host {
				return fc.Type, base
			}
		}
	}
	return "", ""
}
