package editlink

import (
	"fmt"
	"net/url"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// HeuristicDetector detects forge information using host-based heuristics.
type HeuristicDetector struct{}

// NewHeuristicDetector creates a new detector that uses hostname heuristics.
func NewHeuristicDetector() *HeuristicDetector {
	return &HeuristicDetector{}
}

// Name returns the detector name.
func (d *HeuristicDetector) Name() string {
	return "heuristic"
}

// Detect attempts to determine forge type based on hostname patterns.
func (d *HeuristicDetector) Detect(ctx DetectionContext) DetectionResult {
	cloneURL := ctx.CloneURL
	if cloneURL == "" {
		return DetectionResult{Found: false}
	}

	// Skip local file paths (relative or absolute) - they have no web edit URL
	if d.isLocalPath(cloneURL) {
		return DetectionResult{Found: false}
	}

	// Try different heuristic patterns
	forgeType := d.detectForgeTypeFromHost(cloneURL)
	if forgeType == "" {
		return DetectionResult{Found: false}
	}

	// Extract full name
	fullName := d.extractFullNameFromURL(cloneURL)
	if fullName == "" {
		return DetectionResult{Found: false}
	}

	// Determine base URL
	baseURL := d.determineBaseURL(cloneURL, forgeType)

	return DetectionResult{
		ForgeType: forgeType,
		BaseURL:   baseURL,
		FullName:  fullName,
		Found:     true,
	}
}

// detectForgeTypeFromHost determines forge type based on hostname patterns.
func (d *HeuristicDetector) detectForgeTypeFromHost(cloneURL string) config.ForgeType {
	switch {
	case strings.Contains(cloneURL, "github."):
		return config.ForgeGitHub
	case strings.Contains(cloneURL, "gitlab."):
		return config.ForgeGitLab
	case strings.Contains(cloneURL, "bitbucket.org"):
		// Special case: Bitbucket uses custom URL format
		return config.ForgeForgejo // Use Forgejo as placeholder since Bitbucket isn't defined
	case strings.Contains(cloneURL, "forgejo") || strings.Contains(cloneURL, "gitea"):
		return config.ForgeForgejo
	default:
		// Assume Forgejo/Gitea for unknown self-hosted instances
		// This preserves backward compatibility with tests
		return config.ForgeForgejo
	}
}

// extractFullNameFromURL extracts the repository full name (owner/repo) from a URL.
func (d *HeuristicDetector) extractFullNameFromURL(cloneURL string) string {
	normalized := d.normalizeSSHURL(cloneURL)

	u, err := url.Parse(normalized)
	if err != nil {
		return ""
	}

	// Extract path and clean it
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	return path
}

// normalizeSSHURL converts SSH URLs to HTTPS format for easier parsing.
func (d *HeuristicDetector) normalizeSSHURL(repoURL string) string {
	if !strings.HasPrefix(repoURL, "git@") {
		return repoURL
	}

	parts := strings.SplitN(strings.TrimPrefix(repoURL, "git@"), ":", 2)
	if len(parts) == 2 {
		return "https://" + parts[0] + "/" + parts[1]
	}

	return repoURL
}

// determineBaseURL calculates the base URL for a given clone URL and forge type.
func (d *HeuristicDetector) determineBaseURL(cloneURL string, forgeType config.ForgeType) string {
	normalized := d.normalizeSSHURL(cloneURL)

	// Special handling for Bitbucket
	if strings.Contains(cloneURL, "bitbucket.org") {
		return "https://bitbucket.org"
	}

	// Try to extract from the URL
	if u, err := url.Parse(normalized); err == nil && u.Scheme != "" && u.Host != "" {
		return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	// Fallback to public defaults
	switch forgeType {
	case config.ForgeGitHub:
		return "https://github.com"
	case config.ForgeGitLab:
		return "https://gitlab.com"
	case config.ForgeForgejo:
		// For Forgejo/Gitea, use the clone URL as base
		return cloneURL
	case config.ForgeLocal:
		// For local forges, return the clone URL as-is (it's the base)
		return cloneURL
	default:
		return ""
	}
}

// isLocalPath checks if a URL is a local file path (not a remote git URL).
// Returns true for relative paths (./, ../, bare paths) and absolute paths (/, C:\, /home/...).
func (d *HeuristicDetector) isLocalPath(urlStr string) bool {
	// Check for URL schemes that indicate remote repositories
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return false
	}
	if strings.HasPrefix(urlStr, "git@") || strings.HasPrefix(urlStr, "ssh://") {
		return false
	}
	if strings.HasPrefix(urlStr, "git://") {
		return false
	}

	// If it doesn't have a remote scheme, it's a local path
	return true
}
