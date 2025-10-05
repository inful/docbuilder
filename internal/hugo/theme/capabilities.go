package theme

import "git.home.luguber.info/inful/docbuilder/internal/config"

// ThemeCapabilities enumerates optional behavior toggles granted by a theme.
type ThemeCapabilities struct {
	WantsPerPageEditLinks bool // theme UI surfaces per-page edit URLs
	SupportsSearchJSON    bool // theme expects search index JSON module
}

var themeCaps = map[config.Theme]ThemeCapabilities{
	config.ThemeHextra: {WantsPerPageEditLinks: true, SupportsSearchJSON: true},
	config.ThemeDocsy:  {WantsPerPageEditLinks: false, SupportsSearchJSON: true},
}

// Capabilities returns the declared capabilities for a theme (zero value if unknown).
func Capabilities(t config.Theme) ThemeCapabilities { return themeCaps[t] }
