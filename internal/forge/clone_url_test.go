package forge

import "testing"

// TestSplitCloneURL exercises HTTPS, SSH, and edge cases. Pairs with
// TestDetermineBaseURL_and_extractFullName which originally lived in
// editlink/resolver_test.go: splitting must produce the same host + path.
func TestSplitCloneURL(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantBase     string
		wantFullName string
	}{
		{
			name:         "HTTPS github",
			in:           "https://github.com/owner/repo.git",
			wantBase:     "https://github.com",
			wantFullName: "owner/repo",
		},
		{
			name:         "SSH github",
			in:           "git@github.com:owner/repo.git",
			wantBase:     "https://github.com",
			wantFullName: "owner/repo",
		},
		{
			name:         "no .git suffix",
			in:           "https://github.com/owner/repo",
			wantBase:     "https://github.com",
			wantFullName: "owner/repo",
		},
		{
			name:         "nested subgroup gitlab",
			in:           "https://gitlab.example.org/group/sub/repo.git",
			wantBase:     "https://gitlab.example.org",
			wantFullName: "group/sub/repo",
		},
		{
			name:         "bitbucket",
			in:           "https://bitbucket.org/owner/repo.git",
			wantBase:     "https://bitbucket.org",
			wantFullName: "owner/repo",
		},
		{
			name:         "unparseable",
			in:           "://bad url",
			wantBase:     "",
			wantFullName: "",
		},
		{
			name:         "empty",
			in:           "",
			wantBase:     "",
			wantFullName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotName := SplitCloneURL(tt.in)
			if gotBase != tt.wantBase || gotName != tt.wantFullName {
				t.Errorf("SplitCloneURL(%q) = (%q, %q), want (%q, %q)",
					tt.in, gotBase, gotName, tt.wantBase, tt.wantFullName)
			}
		})
	}
}

// TestSplitCloneURL_RoundtripsWithGenerateEditURL ensures that splitting
// and then calling GenerateEditURL with the per-forge types still
// produces the canonical /<fullName>/<suffix>/<branch>/<file> URL.
func TestSplitCloneURL_RoundtripsWithGenerateEditURL(t *testing.T) {
	cases := []struct {
		url      string
		branch   string
		filePath string
		want     string
	}{
		{
			url:      "https://github.com/owner/repo.git",
			branch:   "main",
			filePath: "docs/readme.md",
			want:     "https://github.com/owner/repo/edit/main/docs/readme.md",
		},
		{
			url:      "https://gitlab.example.org/group/sub/repo.git",
			branch:   "main",
			filePath: "guide/intro.md",
			want:     "https://gitlab.example.org/group/sub/repo/-/edit/main/guide/intro.md",
		},
		{
			url:      "https://code.example.org/team/project.git",
			branch:   "feature/x",
			filePath: "docs/page.md",
			want:     "https://code.example.org/team/project/_edit/feature/x/docs/page.md",
		},
	}
	for _, tc := range cases {
		base, name := SplitCloneURL(tc.url)
		if base == "" || name == "" {
			t.Errorf("SplitCloneURL(%q): empty split", tc.url)
			continue
		}
		got := GenerateEditURL(DetectForgeTypeFromURL(tc.url), base, name, tc.branch, tc.filePath)
		if got != tc.want {
			t.Errorf("roundtrip for %q: got %q, want %q", tc.url, got, tc.want)
		}
	}
}
