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
			name:       "Regular file - relative link preserves directory context",
			path:       "sibling.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    false,
			docPath:    "myrepo/section/page.md",
			want:       "/myrepo/section/sibling",
		},
		{
			name:       "Regular file in subdirectory - relative link preserves directory (servejs api.md case)",
			path:       "config.md",
			repository: "servejs",
			forge:      "",
			isIndex:    false,
			docPath:    "content/servejs/reference/api.md",
			want:       "/servejs/reference/config",
		},
		{
			name:       "Regular file in subdirectory - sibling link",
			path:       "other.md",
			repository: "docs",
			forge:      "",
			isIndex:    false,
			docPath:    "docs/guides/tutorial.md",
			want:       "/docs/guides/other",
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
		{
			name:       "Index file with content/ prefix - relative link (servejs case)",
			path:       "tutorials/index.md",
			repository: "servejs",
			forge:      "",
			isIndex:    true,
			docPath:    "content/servejs/_index.md",
			want:       "/servejs/tutorials/",
		},
		{
			name:       "Index file with content/ prefix in subdirectory",
			path:       "configure.md",
			repository: "servejs",
			forge:      "",
			isIndex:    true,
			docPath:    "content/servejs/how-to/_index.md",
			want:       "/servejs/how-to/configure",
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
