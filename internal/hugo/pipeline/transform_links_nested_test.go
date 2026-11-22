package pipeline

import "testing"

// A link that goes one level up is resolved against the directory of the
// document: from "a/b" it points at "a" and not at the root of the
// repository.
func TestParentDirectoryNavigationResolvesAgainstTheDocument(t *testing.T) {
	got := rewriteLinkPath("../config.md", "repo", "", false, "repo/a/b/page.md", false)
	want := "/repo/a/config"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Two levels up go two levels up.
func TestParentDirectoryNavigationGoesUpTwoLevels(t *testing.T) {
	got := rewriteLinkPath("../../config.md", "repo", "", false, "repo/a/b/page.md", false)
	want := "/repo/config"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The content path is lowercased when it is written, so the link is
// lowercased too: the page is published at the lowercased address.
func TestLinksAreLowercased(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		repository string
		forge      string
		isIndex    bool
		docPath    string
		want       string
	}{
		{
			name:       "a repository with a capital letter",
			path:       "other.md",
			repository: "Drift",
			forge:      "",
			isIndex:    false,
			docPath:    "drift/section/page.md",
			want:       "/drift/section/other",
		},
		{
			name:       "a forge with a capital letter",
			path:       "other.md",
			repository: "repo",
			forge:      "GitLab",
			isIndex:    false,
			docPath:    "gitlab/repo/section/page.md",
			want:       "/gitlab/repo/section/other",
		},
		{
			name:       "a file with a capital letter",
			path:       "Other.md",
			repository: "repo",
			forge:      "",
			isIndex:    false,
			docPath:    "repo/section/page.md",
			want:       "/repo/section/other",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteLinkPath(tc.path, tc.repository, tc.forge, tc.isIndex, tc.docPath, false)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
