package urlutil

import "testing"

// TestIsExternalURL covers the lint-side predicate in isolation:
// only http(s) is external. mailto:, tel:, fragments, and relative
// paths are NOT external URLs.
func TestIsExternalURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com/path?x=1#frag", true},
		{"HTTPS://example.com", false}, // case-sensitive by design
		{"mailto:user@example.com", false},
		{"tel:+15551234567", false},
		{"#anchor", false},
		{"./relative/path.md", false},
		{"/abs/path.md", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsExternalURL(tt.in); got != tt.want {
			t.Errorf("IsExternalURL(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestIsForgeURL covers clone-URL detection: http(s), SSH, and git@
// forms are forge URLs. file:// URLs and bare paths are NOT.
func TestIsForgeURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"https://github.com/owner/repo.git", true},
		{"http://git.example.org/repo.git", true},
		{"git@github.com:owner/repo.git", true},
		{"ssh://git@github.com/owner/repo.git", true},
		{"git://github.com/owner/repo.git", true},
		{"./local/path", false},
		{"file:///tmp/local.md", false},
		{"", false},
		{"#fragment", false},
	}
	for _, tt := range tests {
		if got := IsForgeURL(tt.in); got != tt.want {
			t.Errorf("IsForgeURL(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestIsLocalPath is the inverse of IsForgeURL -- same predicates,
// inverted.
func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"./relative/path.md", true},
		{"/abs/path.md", true},
		{"docs/page.md", true},
		{"https://github.com/owner/repo.git", false},
		{"git@github.com:owner/repo.git", false},
		{"ssh://git@github.com/repo.git", false},
		{"", true}, // empty is not a forge URL, so it is local by definition
	}
	for _, tt := range tests {
		if got := IsLocalPath(tt.in); got != tt.want {
			t.Errorf("IsLocalPath(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestIsAbsoluteOrSpecialURL captures the "pass through unchanged" set
// the markdown link rewriter respects. http(s), fragment, mailto, tel.
func TestIsAbsoluteOrSpecialURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"#section", true},
		{"#", true},
		{"mailto:user@example.com", true},
		{"tel:+15551234567", true},
		{"./relative", false},
		{"/abs/path", false},
		{"docs/page.md", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsAbsoluteOrSpecialURL(tt.in); got != tt.want {
			t.Errorf("IsAbsoluteOrSpecialURL(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
