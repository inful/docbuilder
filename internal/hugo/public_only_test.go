package hugo

import "testing"

func TestIsPublicMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "no frontmatter",
			content: "# Title\n\nBody\n",
			want:    false,
		},
		{
			name:    "empty frontmatter",
			content: "---\n---\n# Title\n",
			want:    false,
		},
		{
			name:    "public true",
			content: "---\npublic: true\n---\n# Title\n",
			want:    true,
		},
		{
			name:    "public false",
			content: "---\npublic: false\n---\n# Title\n",
			want:    false,
		},
		{
			name:    "public string true",
			content: "---\npublic: \"true\"\n---\n# Title\n",
			want:    false,
		},
		{
			name:    "invalid yaml",
			content: "---\n:bad yaml\n---\n# Title\n",
			want:    false,
		},
		{
			name:    "missing closing delimiter",
			content: "---\npublic: true\n# Title\n",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPublicMarkdown([]byte(tt.content)); got != tt.want {
				t.Fatalf("isPublicMarkdown()=%v, want %v", got, tt.want)
			}
		})
	}
}
