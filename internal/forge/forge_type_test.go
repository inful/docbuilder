package forge

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestDetectForgeTypeFromURL covers the hostname-pattern detection used by
// edit-link resolvers and the content pipeline. Centralized here so both
// call sites see identical behavior.
func TestDetectForgeTypeFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want config.ForgeType
	}{
		{
			name: "github.com",
			url:  "https://github.com/org/repo.git",
			want: config.ForgeGitHub,
		},
		{
			name: "self-hosted github enterprise",
			url:  "https://github.example.com/org/repo.git",
			want: config.ForgeGitHub,
		},
		{
			name: "gitlab.com",
			url:  "https://gitlab.com/org/repo.git",
			want: config.ForgeGitLab,
		},
		{
			name: "self-hosted gitlab",
			url:  "https://gitlab.example.org/org/repo.git",
			want: config.ForgeGitLab,
		},
		{
			name: "forgejo.org",
			url:  "https://codeberg.org/forgejo/forgejo.git",
			want: config.ForgeForgejo,
		},
		{
			name: "gitea.io",
			url:  "https://gitea.io/org/repo.git",
			want: config.ForgeForgejo,
		},
		{
			name: "bitbucket is treated as Forgejo placeholder",
			url:  "https://bitbucket.org/org/repo.git",
			want: config.ForgeForgejo,
		},
		{
			name: "unknown self-hosted defaults to Forgejo",
			url:  "https://git.example.com/org/repo.git",
			want: config.ForgeForgejo,
		},
		{
			name: "empty input defaults to Forgejo",
			url:  "",
			want: config.ForgeForgejo,
		},
		{
			name: "SSH git@github.com:org/repo.git",
			url:  "git@github.com:org/repo.git",
			want: config.ForgeGitHub,
		},
		{
			name: "SSH git@gitlab.com:org/repo.git",
			url:  "git@gitlab.com:org/repo.git",
			want: config.ForgeGitLab,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectForgeTypeFromURL(tt.url); got != tt.want {
				t.Errorf("DetectForgeTypeFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
