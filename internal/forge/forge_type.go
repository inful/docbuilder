package forge

import (
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// DetectForgeTypeFromURL classifies a (clone) URL by host pattern.
//
// Behavior:
//   - Strings containing "github." resolve to ForgeGitHub.
//   - Strings containing "gitlab." resolve to ForgeGitLab.
//   - Strings containing "forgejo", "gitea", or "bitbucket.org" resolve to
//     ForgeForgejo. (Bitbucket is not yet a first-class ForgeType; we map
//     it to the closest existing pattern so callers at least get a URL.)
//   - Anything else resolves to ForgeForgejo (the most-common self-hosted
//     option today). Empty input follows the same rule.
//
// Use this when callers don't have an explicit `forge` metadata field and
// can only inspect URLs. When callers DO have explicit forge metadata,
// prefer that (see pipeline.detectForgeType which consults the field
// first and falls back here).
func DetectForgeTypeFromURL(rawURL string) config.ForgeType {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lower, "github."):
		return config.ForgeGitHub
	case strings.Contains(lower, "gitlab."):
		return config.ForgeGitLab
	case strings.Contains(lower, "forgejo"),
		strings.Contains(lower, "gitea"),
		strings.Contains(lower, "bitbucket.org"):
		return config.ForgeForgejo
	default:
		return config.ForgeForgejo
	}
}
