package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteLinkPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		repository string
		forge      string
		isIndex    bool
		docPath    string
		want       string
	}{
		{
			name:       "Index file in subdirectory - relative link preserves directory",
			path:       "configure-env-exposure.md",
			repository: "servejs",
			forge:      "",
			isIndex:    true,
			docPath:    "servejs/how-to/_index.md",
			want:       "/servejs/how-to/configure-env-exposure",
		},
		{
			name:       "Index file in subdirectory with forge - relative link preserves directory",
			path:       "authentication.md",
			repository: "docs",
			forge:      "gitlab",
			isIndex:    true,
			docPath:    "gitlab/docs/how-to/_index.md",
			want:       "/gitlab/docs/how-to/authentication",
		},
		{
			name:       "Index file at repository root - relative link",
			path:       "getting-started.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    true,
			docPath:    "myrepo/_index.md",
			want:       "/myrepo/getting-started",
		},
		{
			name:       "Index file with ../ navigation",
			path:       "../other-section/file.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    true,
			docPath:    "myrepo/section/_index.md",
			want:       "/myrepo/other-section/file",
		},
		{
			name:       "Regular file - relative link gets repository prefix",
			path:       "sibling.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    false,
			docPath:    "myrepo/section/page.md",
			want:       "/myrepo/sibling",
		},
		{
			name:       "Index file - subdirectory link",
			path:       "guide/setup.md",
			repository: "docs",
			forge:      "",
			isIndex:    true,
			docPath:    "docs/_index.md",
			want:       "/docs/guide/setup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteLinkPath(tt.path, tt.repository, tt.forge, tt.isIndex, tt.docPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractDirectory(t *testing.T) {
	tests := []struct {
		name     string
		hugoPath string
		want     string
	}{
		{
			name:     "Index in subdirectory",
			hugoPath: "servejs/how-to/_index.md",
			want:     "how-to",
		},
		{
			name:     "Index at repository root",
			hugoPath: "servejs/_index.md",
			want:     "",
		},
		{
			name:     "Regular file in subdirectory",
			hugoPath: "myrepo/api/reference.md",
			want:     "api",
		},
		{
			name:     "Nested subdirectory",
			hugoPath: "myproject/guide/advanced/_index.md",
			want:     "guide/advanced",
		},
		{
			name:     "File with forge namespace",
			hugoPath: "gitlab/myrepo/how-to/index.md",
			want:     "how-to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDirectory(tt.hugoPath)
			assert.Equal(t, tt.want, got)
		})
	}
}
