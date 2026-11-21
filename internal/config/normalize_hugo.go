package config

import "fmt"

// normalizeHugoConfig canonicalizes Hugo-related enum fields (today:
// the theme and the sidebar mode) and reports any coercions via res.
func normalizeHugoConfig(h *HugoConfig, res *NormalizationResult) {
	if h == nil {
		return
	}
	h.Sidebar.Normalize(res)
	switch h.Theme {
	case "", ThemeRelearn, ThemeDocsy:
		// Known values; nothing to normalize.
	default:
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"hugo.theme %q is unknown; falling back to %q",
			h.Theme, ThemeRelearn,
		))
		h.Theme = ThemeRelearn
	}
}
