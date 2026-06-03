package config

// normalizeHugoConfig canonicalizes Hugo-related enum fields (today:
// the sidebar mode) and reports any coercions via res.
func normalizeHugoConfig(h *HugoConfig, res *NormalizationResult) {
	if h == nil {
		return
	}
	h.Sidebar.Normalize(res)
}
