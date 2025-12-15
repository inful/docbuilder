package theme

import "git.home.luguber.info/inful/docbuilder/internal/config"

// Capabilities enumerates optional behavior toggles granted by a theme.
type Capabilities struct {
	WantsPerPageEditLinks bool // theme UI surfaces per-page edit URLs
	SupportsSearchJSON    bool // theme expects search index JSON module
}

var themeCaps = map[config.Theme]Capabilities{
	config.ThemeHextra:  {WantsPerPageEditLinks: true, SupportsSearchJSON: true},
	config.ThemeDocsy:   {WantsPerPageEditLinks: true, SupportsSearchJSON: true},
	config.ThemeRelearn: {WantsPerPageEditLinks: true, SupportsSearchJSON: true},
}

// GetCapabilities returns the declared capabilities for a theme (zero value if unknown).
func GetCapabilities(t config.Theme) Capabilities { return themeCaps[t] }
