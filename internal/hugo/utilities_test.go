package hugo

import "testing"

// TestTitleCase covers the simple title-casing helper in utilities.go.
func TestTitleCase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single word", "hello", "Hello"},
		{"two words", "hello world", "Hello World"},
		{"uppercase preserved lowercase tail", "hELLO wORLD", "Hello World"},
		{"hyphen preserved", "my-repo", "My-repo"},
		{"already title case", "Hello World", "Hello World"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TitleCase(tt.in); got != tt.want {
				t.Errorf("TitleCase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestTitleCaseSlug covers the slug-aware variant.
func TestTitleCaseSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"hyphen slug", "my-docs-repo", "My Docs Repo"},
		{"underscore slug", "my_docs_repo", "My Docs Repo"},
		{"mixed", "my-docs_repo", "My Docs Repo"},
		{"plain word", "docs", "Docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TitleCaseSlug(tt.in); got != tt.want {
				t.Errorf("TitleCaseSlug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
