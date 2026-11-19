package lint

import "testing"

// The lint excludes are globs. A wrong glob means the wrong files are
// skipped: too much and the linter misses files, too little and it
// scans files the operator wanted out of the way.
func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		name string
		glob string
		path string
		want bool
	}{
		{name: "exact match", glob: "README.md", path: "README.md", want: true},
		{name: "no match", glob: "README.md", path: "docs/README.md", want: false},
		{name: "star does not cross a slash", glob: "*.md", path: "docs/a.md", want: false},
		{name: "star in the same folder", glob: "*.md", path: "a.md", want: true},
		{name: "double star crosses folders", glob: "docs/**", path: "docs/a/b.md", want: true},
		{name: "double star in the same folder", glob: "docs/**", path: "docs/a.md", want: true},
		{name: "double star needs the prefix", glob: "docs/**", path: "other/a.md", want: false},
		{name: "question mark is one character", glob: "a?c.md", path: "abc.md", want: true},
		{name: "question mark is not two", glob: "a?c.md", path: "abbc.md", want: false},
		{name: "a dot is a dot", glob: "a.b.md", path: "a.b.md", want: true},
		{name: "a dot is not any character", glob: "a.b.md", path: "axb.md", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := globToRegexp(tt.glob)
			if got := re.MatchString(tt.path); got != tt.want {
				t.Errorf("globToRegexp(%q).MatchString(%q) = %v, want %v", tt.glob, tt.path, got, tt.want)
			}
		})
	}
}
