package config

import (
    "git.home.luguber.info/inful/docbuilder/internal/foundation/normalization"
)

// VersioningStrategy enumerates supported multi-version selection strategies.
type VersioningStrategy string

const (
    StrategyBranchesAndTags VersioningStrategy = "branches_and_tags"
    StrategyBranchesOnly    VersioningStrategy = "branches_only"
    StrategyTagsOnly        VersioningStrategy = "tags_only"
)

// NormalizeVersioningStrategy returns a canonical typed strategy or empty string if unknown.
var versioningStrategyNormalizer = normalization.NewNormalizer(map[string]VersioningStrategy{
    "branches_and_tags": StrategyBranchesAndTags,
    "branches_only":     StrategyBranchesOnly,
    "tags_only":         StrategyTagsOnly,
}, StrategyBranchesAndTags)

func NormalizeVersioningStrategy(raw string) VersioningStrategy {
    return versioningStrategyNormalizer.Normalize(raw)
}
