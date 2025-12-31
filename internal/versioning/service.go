package versioning

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// GetVersioningConfig creates a VersionConfig from V2Config.
func GetVersioningConfig(v2Config *config.Config) *VersionConfig {
	if v2Config.Versioning == nil {
		// Return default configuration
		return &VersionConfig{
			Strategy:          StrategyDefaultOnly,
			DefaultBranchOnly: true,
			BranchPatterns:    []string{"main", "master"},
			TagPatterns:       []string{},
			MaxVersions:       5,
		}
	}

	var strategy VersionStrategy
	switch v2Config.Versioning.Strategy {
	case config.StrategyBranchesOnly:
		strategy = StrategyBranches
	case config.StrategyTagsOnly:
		strategy = StrategyTags
	case config.StrategyBranchesAndTags:
		strategy = StrategyBranchesAndTags
	default:
		// Unknown or empty strategy - default to StrategyDefaultOnly
		strategy = StrategyDefaultOnly
	}

	return &VersionConfig{
		Strategy:          strategy,
		DefaultBranchOnly: v2Config.Versioning.DefaultBranchOnly,
		BranchPatterns:    v2Config.Versioning.BranchPatterns,
		TagPatterns:       v2Config.Versioning.TagPatterns,
		MaxVersions:       v2Config.Versioning.MaxVersionsPerRepo,
	}
}
