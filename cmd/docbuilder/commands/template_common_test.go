package commands

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestResolveTemplateBaseURL(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			BaseURL: "https://docs.example.com",
		},
	}

	t.Run("flag wins", func(t *testing.T) {
		t.Setenv("DOCBUILDER_TEMPLATE_BASE_URL", "https://env.example.com")
		url, err := ResolveTemplateBaseURL("https://flag.example.com", cfg)
		require.NoError(t, err)
		require.Equal(t, "https://flag.example.com", url)
	})

	t.Run("env wins over config", func(t *testing.T) {
		t.Setenv("DOCBUILDER_TEMPLATE_BASE_URL", "https://env.example.com")
		url, err := ResolveTemplateBaseURL("", cfg)
		require.NoError(t, err)
		require.Equal(t, "https://env.example.com", url)
	})

	t.Run("config fallback", func(t *testing.T) {
		t.Setenv("DOCBUILDER_TEMPLATE_BASE_URL", "")
		url, err := ResolveTemplateBaseURL("", cfg)
		require.NoError(t, err)
		require.Equal(t, "https://docs.example.com", url)
	})

	t.Run("missing base URL", func(t *testing.T) {
		t.Setenv("DOCBUILDER_TEMPLATE_BASE_URL", "")
		url, err := ResolveTemplateBaseURL("", &config.Config{})
		require.Error(t, err)
		require.Empty(t, url)
	})
}
