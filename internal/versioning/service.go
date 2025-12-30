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

	strategy := StrategyDefaultOnly
	switch v2Config.Versioning.Strategy {
	case "branches":
		strategy = StrategyBranches
	case "tags":
		strategy = StrategyTags
	case "branches_and_tags":
		strategy = StrategyBranchesAndTags
	}

	return &VersionConfig{
		Strategy:          strategy,
		DefaultBranchOnly: v2Config.Versioning.DefaultBranchOnly,
		BranchPatterns:    v2Config.Versioning.BranchPatterns,
		TagPatterns:       v2Config.Versioning.TagPatterns,
		MaxVersions:       v2Config.Versioning.MaxVersionsPerRepo,
	}
}
