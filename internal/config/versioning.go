package config

import "strings"

// VersioningStrategy enumerates supported multi-version selection strategies.
type VersioningStrategy string

const (
    StrategyBranchesAndTags VersioningStrategy = "branches_and_tags"
    StrategyBranchesOnly    VersioningStrategy = "branches_only"
    StrategyTagsOnly        VersioningStrategy = "tags_only"
)

// NormalizeVersioningStrategy returns a canonical typed strategy or empty string if unknown.
func NormalizeVersioningStrategy(raw string) VersioningStrategy {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    case string(StrategyBranchesAndTags):
        return StrategyBranchesAndTags
    case string(StrategyBranchesOnly):
        return StrategyBranchesOnly
    case string(StrategyTagsOnly):
        return StrategyTagsOnly
    default:
        return ""
    }
}
