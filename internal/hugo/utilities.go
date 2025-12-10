package hugo

import "strings"

// titleCase converts a string to title case (portable alternative to strings.Title)
func titleCase(s string) string {
	if s == "" {
		return s
	}
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// TitleCase exported helper for theme packages.
func TitleCase(s string) string { return titleCase(s) }

// urlSlug converts a filename to a URL-safe slug (lowercase, spaces to hyphens)
// This matches Hugo's URL generation behavior
func urlSlug(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)
	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	// Could add more URL sanitization here if needed
	return s
}
