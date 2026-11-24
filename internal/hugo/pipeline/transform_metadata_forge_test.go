package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A self-hosted GitHub is "github.something". Only "github.com" was
// recognized, so a GitHub Enterprise was taken for a Forgejo and its
// edit links pointed at the wrong place.
func TestTheForgeOfTheHostIsRecognized(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    config.ForgeType
	}{
		{"github.com", "https://github.com/org/repo", config.ForgeGitHub},
		{"a self-hosted github", "https://github.mycorp.com/org/repo", config.ForgeGitHub},
		{"gitlab.com", "https://gitlab.com/org/repo", config.ForgeGitLab},
		{"a self-hosted gitlab", "https://gitlab.mycorp.com/org/repo", config.ForgeGitLab},
		{"forgejo", "https://git.example.com/org/repo", config.ForgeForgejo},
		{"gitea", "https://gitea.example.com/org/repo", config.ForgeForgejo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectForgeType("", tt.baseURL))
		})
	}
}
