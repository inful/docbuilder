package config

import "strings"

// ForgeType enumerates supported forge providers.
type ForgeType string

const (
    ForgeGitHub  ForgeType = "github"
    ForgeGitLab  ForgeType = "gitlab"
    ForgeForgejo ForgeType = "forgejo"
)

// NormalizeForgeType canonicalizes a forge type string (case-insensitive) or returns empty if unknown.
func NormalizeForgeType(raw string) ForgeType {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    case string(ForgeGitHub):
        return ForgeGitHub
    case string(ForgeGitLab):
        return ForgeGitLab
    case string(ForgeForgejo):
        return ForgeForgejo
    default:
        return ""
    }
}
