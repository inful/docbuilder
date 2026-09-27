package lint

import (
	"path/filepath"
	"testing"
)

// TestDefaultExcludes_ContainsExpected verifies the cross-forge union of
// git-forge-conventional files is in DefaultExcludes.
func TestDefaultExcludes_ContainsExpected(t *testing.T) {
	expected := []string{
		"README.md",
		"CONTRIBUTING.md",
		"SECURITY.md",
		"CODE_OF_CONDUCT.md",
		"SUPPORT.md",
		"CHANGELOG.md",
		"LICENSE",
		"LICENSE.md",
		"AUTHORS.md",
		"NOTICE.md",
		".github/PULL_REQUEST_TEMPLATE.md",
		".gitlab/issue_templates/*.md",
		".gitea/ISSUE_TEMPLATE/**/*.md",
	}

	for _, e := range expected {
		found := false
		for _, p := range DefaultExcludes {
			if p == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultExcludes missing expected pattern %q", e)
		}
	}
}

// TestCompileGlobs_ValidPatterns verifies all default patterns compile.
func TestCompileGlobs_ValidPatterns(t *testing.T) {
	globs := compileGlobs(DefaultExcludes)
	if len(globs) != len(DefaultExcludes) {
		t.Fatalf("compileGlobs produced %d patterns from %d inputs", len(globs), len(DefaultExcludes))
	}
}

// TestMatchGlob_ExactMatch verifies exact globs only match at top level.
func TestMatchGlob_ExactMatch(t *testing.T) {
	g := compileGlob("README.md")

	cases := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"docs/README.md", false}, // exact glob doesn't recurse
		{"docs/inner/README.md", false},
		{"CONTRIBUTING.md", false},
	}

	for _, tc := range cases {
		got := matchGlob(g, tc.path)
		if got != tc.want {
			t.Errorf("matchGlob(README.md, %q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestMatchGlob_RecursiveMatch verifies ** globs match at any depth.
func TestMatchGlob_RecursiveMatch(t *testing.T) {
	g := compileGlob(".github/ISSUE_TEMPLATE/**/*.md")

	cases := []struct {
		path string
		want bool
	}{
		{".github/ISSUE_TEMPLATE/bug.md", true},
		{".github/ISSUE_TEMPLATE/sub/foo.md", true},
		{".github/ISSUE_TEMPLATE/sub/deep/foo.md", true},
		{".github/PULL_REQUEST_TEMPLATE.md", false},
		{"docs/foo.md", false},
	}

	for _, tc := range cases {
		got := matchGlob(g, tc.path)
		if got != tc.want {
			t.Errorf("matchGlob(.github/ISSUE_TEMPLATE/**/*.md, %q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestMatchGlob_RecursiveWithoutPrefix verifies ** at start matches anything.
func TestMatchGlob_RecursiveWithoutPrefix(t *testing.T) {
	g := compileGlob("**/foo.md")

	cases := []struct {
		path string
		want bool
	}{
		{"foo.md", true},
		{"docs/foo.md", true},
		{"docs/sub/foo.md", true},
		{"docs/bar.md", false},
	}

	for _, tc := range cases {
		got := matchGlob(g, tc.path)
		if got != tc.want {
			t.Errorf("matchGlob(**/foo.md, %q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestLinter_ExcludesDefaultFiles verifies the linter excludes git-forge-conventional files.
func TestLinter_ExcludesDefaultFiles(t *testing.T) {
	linter := NewLinter(&Config{Format: formatText})

	cases := []string{
		"/repo/README.md",
		"/repo/CONTRIBUTING.md",
		"/repo/LICENSE",
		"/repo/LICENSE.md",
		"/repo/SECURITY.md",
		"/repo/.github/PULL_REQUEST_TEMPLATE.md",
	}

	for _, p := range cases {
		if !linter.isExcluded(p, "/repo") {
			t.Errorf("isExcluded(%q) = false, want true", p)
		}
	}
}

// TestLinter_ExcludesCustomConfig verifies a custom Excludes list is honored.
func TestLinter_ExcludesCustomConfig(t *testing.T) {
	linter := NewLinter(&Config{
		Format:   formatText,
		Excludes: []string{"CUSTOM.md"}, // explicit empty default override not applied
	})

	// With explicit empty list, no defaults apply — but custom should still match.
	if !linter.isExcluded("/repo/CUSTOM.md", "/repo") {
		t.Errorf("isExcluded(/repo/CUSTOM.md) = false, want true")
	}
	// README.md should NOT match when Excludes is explicitly set (overriding defaults)
	if linter.isExcluded("/repo/README.md", "/repo") {
		t.Errorf("isExcluded(/repo/README.md) = true with custom Excludes; want false")
	}
}

// TestLinter_NoExcludes verifies linter with nil Excludes uses defaults.
func TestLinter_NoExcludes(t *testing.T) {
	linter := NewLinter(&Config{Format: formatText})

	if !linter.isExcluded("/repo/README.md", "/repo") {
		t.Errorf("isExcluded(/repo/README.md) = false with nil Excludes; want true (defaults)")
	}
}

// TestIsExcludedPath_FixerStyle verifies fixer-style usage of exclude logic.
func TestIsExcludedPath_FixerStyle(t *testing.T) {
	globs := compileGlobs(DefaultExcludes)

	cases := []struct {
		path     string
		baseDir  string
		want     bool
	}{
		{filepath.Join("/repo", "README.md"), "/repo", true},
		{filepath.Join("/repo", "docs", "README.md"), "/repo", false},
		{filepath.Join("/repo", ".github", "PULL_REQUEST_TEMPLATE.md"), "/repo", true},
		{filepath.Join("/repo", "CONTRIBUTING.md"), "/repo", true},
	}

	for _, tc := range cases {
		got := isExcludedPath(tc.path, tc.baseDir, globs)
		if got != tc.want {
			t.Errorf("isExcludedPath(%q, %q) = %v, want %v", tc.path, tc.baseDir, got, tc.want)
		}
	}
}
