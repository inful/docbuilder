package lint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The name of the file is replaced in the last segment of the link.
//
// The replacement of the first occurrence of the name anywhere in the
// target changed the directory of the link ("../old-name/old-name.md")
// and the query of the link ("old-name.md?x=old-name"): the link then
// pointed at a page that does not exist.
func TestTheFallbackRewriteOnlyTouchesTheLastSegment(t *testing.T) {
	fixer := &Fixer{}

	cases := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "the directory is not the name of the file",
			target: "../old-name/old-name.md",
			want:   "../old-name/new-name.md",
		},
		{
			name:   "the directory holds the name of the file",
			target: "old-name.md/notes.md",
			want:   "old-name.md/notes.md",
		},
		{
			name:   "the query is not the name of the file",
			target: "old-name.md?x=old-name",
			want:   "new-name.md?x=old-name",
		},
		{
			name:   "the image of the file",
			target: "./images/old-name.md.png",
			want:   "./images/new-name.md.png",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fixer.updateLinkTarget(
				LinkReference{Target: tc.target},
				"old-name.md",
				"new-name.md",
			)
			require.Equal(t, tc.want, got)
		})
	}
}
