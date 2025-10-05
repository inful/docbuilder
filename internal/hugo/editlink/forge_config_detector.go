package editlink

import (
	"net/url"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// ForgeConfigDetector detects forge information by matching against configured forge base URLs.
type ForgeConfigDetector struct{}

// NewForgeConfigDetector creates a new detector that uses forge configuration.
func NewForgeConfigDetector() *ForgeConfigDetector {
	return &ForgeConfigDetector{}
}

// Name returns the detector name.
func (d *ForgeConfigDetector) Name() string {
	return "forge_config"
}

// Detect attempts to match the repository URL against configured forges.
func (d *ForgeConfigDetector) Detect(ctx DetectionContext) DetectionResult {
	forgeType, baseURL := d.resolveForgeForRepository(ctx.Config, ctx.CloneURL)
	if forgeType == "" {
		return DetectionResult{Found: false}
	}

	// Extract full name from the URL
	fullName := d.extractFullNameFromURL(ctx.CloneURL)
	if fullName == "" {
		return DetectionResult{Found: false}
	}

	return DetectionResult{
		ForgeType: forgeType,
		BaseURL:   baseURL,
		FullName:  fullName,
		Found:     true,
	}
}

// resolveForgeForRepository attempts to match a repository clone URL against configured forge base URLs.
func (d *ForgeConfigDetector) resolveForgeForRepository(cfg *config.Config, repoURL string) (config.ForgeType, string) {
	if cfg == nil || len(cfg.Forges) == 0 || repoURL == "" {
		return "", ""
	}

	normalized := d.normalizeSSHURL(repoURL)

	for _, fc := range cfg.Forges {
		if fc == nil || fc.BaseURL == "" {
			continue
		}

		base := strings.TrimSuffix(fc.BaseURL, "/")

		// Direct prefix match
		if strings.HasPrefix(normalized, base+"/") || strings.HasPrefix(normalized, base) {
			return fc.Type, base
		}

		// Host-based match
		if d.hostsMatch(base, normalized) {
			return fc.Type, base
		}
	}

	return "", ""
}

// normalizeSSHURL converts SSH URLs to HTTPS format for easier comparison.
func (d *ForgeConfigDetector) normalizeSSHURL(repoURL string) string {
	if !strings.HasPrefix(repoURL, "git@") {
		return repoURL
	}

	parts := strings.SplitN(strings.TrimPrefix(repoURL, "git@"), ":", 2)
	if len(parts) == 2 {
		return "https://" + parts[0] + "/" + parts[1]
	}

	return repoURL
}

// hostsMatch checks if two URLs have the same host.
func (d *ForgeConfigDetector) hostsMatch(url1, url2 string) bool {
	u1, err1 := url.Parse(url1)
	u2, err2 := url.Parse(url2)

	if err1 != nil || err2 != nil {
		return false
	}

	return u1.Host != "" && u1.Host == u2.Host
}

// extractFullNameFromURL extracts the repository full name (owner/repo) from a URL.
func (d *ForgeConfigDetector) extractFullNameFromURL(cloneURL string) string {
	normalized := d.normalizeSSHURL(cloneURL)

	u, err := url.Parse(normalized)
	if err != nil {
		return ""
	}

	// Extract path and clean it
	path := strings.Trim(u.Path, "/")
	if strings.HasSuffix(path, ".git") {
		path = path[:len(path)-4]
	}

	return path
}
