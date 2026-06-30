package hugo

import "strings"

// TitleCase converts a string to title case (portable alternative to strings.Title).
// The titlecased form is space-separated: each word's first letter is uppercased,
// remaining letters are lowercased.
func TitleCase(s string) string {
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

// TitleCaseSlug is TitleCase's slug-aware variant: '-' and '_' are first
// replaced with spaces, then the standard TitleCase rules apply. Useful for
// repo names like "my-docs-repo" → "My Docs Repo".
func TitleCaseSlug(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return TitleCase(s)
}

// titleCase is the lowercase alias kept so templates and helper files
// in this package can refer to the helper without an import edge. It
// matches TitleCase byte-for-byte.
func titleCase(s string) string { return TitleCase(s) }
