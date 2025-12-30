package hugo

// mergeParams deep-merges src into dst (map[string]any).
// - Maps: merged recursively
// - Slices & scalars: replaced.
func mergeParams(dst, src map[string]any) {
	if src == nil {
		return
	}
	for k, v := range src {
		if mv, ok := v.(map[string]any); ok {
			if existing, ok2 := dst[k].(map[string]any); ok2 {
				mergeParams(existing, mv)
			} else {
				cp := map[string]any{}
				mergeParams(cp, mv)
				dst[k] = cp
			}
			continue
		}
		dst[k] = v
	}
}
