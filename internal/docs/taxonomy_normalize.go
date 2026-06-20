package docs

import (
	"sort"
	"strings"
)

// NormalizeTaxonomyValues trims whitespace from each entry, drops empty
// values, deduplicates case-insensitively (preserving the first-seen
// casing), and returns the result sorted case-insensitively. The input
// is not mutated. Returns nil for an empty or whitespace-only input.
func NormalizeTaxonomyValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}
