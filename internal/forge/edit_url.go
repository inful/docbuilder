package forge

import (
	"fmt"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// GenerateEditURL constructs a web UI edit URL for a repository file given the forge type.
// baseURL should be the canonical web base (no trailing slash), fullName is "org/repo".
// filePath should use forward slashes. Returns empty string if inputs insufficient or unsupported forge type.
func GenerateEditURL(forgeType config.ForgeType, baseURL, fullName, branch, filePath string) string {
	if forgeType == "" || baseURL == "" || fullName == "" || branch == "" || filePath == "" {
		return ""
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	switch forgeType {
	case config.ForgeGitHub:
		return fmt.Sprintf("%s/%s/edit/%s/%s", baseURL, fullName, branch, filePath)
	case config.ForgeGitLab:
		return fmt.Sprintf("%s/%s/-/edit/%s/%s", baseURL, fullName, branch, filePath)
	case config.ForgeForgejo:
		return fmt.Sprintf("%s/%s/_edit/%s/%s", baseURL, fullName, branch, filePath)
	case config.ForgeLocal:
		// Local forges don't have web UI edit URLs
		return ""
	default:
		return ""
	}
}
