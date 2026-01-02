package versioning

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

const (
	// defaultMaxVersions is the default maximum number of versions to keep per repository
	// when no versioning configuration is provided.
	defaultMaxVersions = 5
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
			MaxVersions:       defaultMaxVersions,
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
