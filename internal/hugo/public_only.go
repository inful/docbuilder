package hugo

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

func isDaemonPublicOnlyEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Daemon != nil && cfg.Daemon.Content.PublicOnly
}

// isPublicMarkdown returns true if and only if the input has YAML frontmatter
// with a boolean field `public: true`.
//
// Contract (matches ADR-019):
// - Missing frontmatter => not public
// - Invalid YAML or malformed frontmatter delimiters => not public
// - `public` must be boolean true (not string "true").
func isPublicMarkdown(content []byte) bool {
	fields, _, had, _, err := frontmatterops.Read(content)
	if err != nil {
		return false
	}
	if !had {
		return false
	}
	v, ok := fields["public"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
