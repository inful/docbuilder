package config

import (
	"fmt"
	"strings"
)

// normalizeStringSlice performs trim, dedupe, and sort operations on a string slice.
// It logs warnings when changes occur. Use this for configuration fields that should
// be canonical (trimmed, unique, sorted).
func normalizeStringSlice(label string, in []string, res *NormalizationResult) []string {
	if len(in) == 0 {
		return in
	}

	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	changed := false

	// Trim, dedupe
	for _, v := range in {
		t := strings.TrimSpace(v)
		if t == "" {
			changed = true
			continue
		}
		if _, ok := seen[t]; ok {
			changed = true
			continue
		}
		if t != v {
			changed = true
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}

	if changed {
		res.Warnings = append(res.Warnings, fmt.Sprintf("normalized %s list (%d -> %d entries)", label, len(in), len(out)))
	}

	// Sort (insertion sort for small slices)
	if len(out) <= 1 {
		return out
	}
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j-1] > out[j] {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}

	return out
}

// trimStringSlice removes empty entries (after trimming whitespace) from a string slice.
// Does not dedupe or sort. Use this for order-sensitive configuration fields.
func trimStringSlice(in []string) []string {
	if len(in) == 0 {
		return in
	}

	out := make([]string, 0, len(in))
	for _, p := range in {
		if tp := strings.TrimSpace(p); tp != "" {
			out = append(out, tp)
		}
	}
	return out
}
