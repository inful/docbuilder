package config

import "fmt"

// The chrome modes of the theme.
const (
	// ChromeFull renders the navigation chrome on every page. It is
	// the default: the navigation is there without JavaScript.
	ChromeFull = "full"
	// ChromeShared renders the chrome on the donor page only and
	// restores it in the browser. The published site is much
	// smaller, and the navigation needs JavaScript.
	ChromeShared = "shared"
)

// normalizeHugoConfig canonicalizes Hugo-related enum fields (today:
// the chrome mode) and reports any coercions via res.
func normalizeHugoConfig(h *HugoConfig, res *NormalizationResult) {
	if h == nil {
		return
	}
	switch h.Chrome {
	case "", ChromeFull, ChromeShared:
		// Known values; nothing to normalize.
	default:
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"hugo.chrome %q is unknown; falling back to %q",
			h.Chrome, ChromeFull,
		))
		h.Chrome = ChromeFull
	}
}
