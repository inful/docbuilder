package config

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/normalization"
)

// ForgeType enumerates supported forge providers.
type ForgeType string

const (
	ForgeGitHub  ForgeType = "github"
	ForgeGitLab  ForgeType = "gitlab"
	ForgeForgejo ForgeType = "forgejo"
)

// NormalizeForgeType canonicalizes a forge type string (case-insensitive) or returns empty if unknown.
var forgeTypeStringNormalizer = normalization.NewNormalizer(map[string]ForgeType{
	"github":  ForgeGitHub,
	"gitlab":  ForgeGitLab,
	"forgejo": ForgeForgejo,
}, ForgeGitHub)

// NormalizeForgeType canonicalizes a forge type string (case-insensitive) or returns empty if unknown.
func NormalizeForgeType(raw string) ForgeType {
	return forgeTypeStringNormalizer.Normalize(raw)
}

// IsValid reports whether the ForgeType is a known value.
func (f ForgeType) IsValid() bool {
	return NormalizeForgeType(string(f)) != ""
}
