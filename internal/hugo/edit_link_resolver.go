package hugo

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/editlink"
)

// EditLinkResolver encapsulates logic for deriving per-page edit links based on configuration and theme features.
// It is intentionally stateless aside from holding a reference to config; future enhancements (e.g. caching repo lookups)
// can be added without changing call sites.
//
// This has been refactored to use a chain of responsibility pattern for better maintainability and testability.
type EditLinkResolver struct {
	cfg      *config.Config
	resolver *editlink.Resolver
}

// NewEditLinkResolver creates a new resolver bound to the provided config (nil-safe).
func NewEditLinkResolver(cfg *config.Config) *EditLinkResolver {
	return &EditLinkResolver{
		cfg:      cfg,
		resolver: editlink.NewResolver(),
	}
}

// Resolve returns the edit URL for the provided doc file or empty string if one should not be generated.
// This maintains the same interface as the original implementation while using the new decomposed logic.
func (r *EditLinkResolver) Resolve(file docs.DocFile) string {
	return r.resolver.Resolve(file, r.cfg)
}
