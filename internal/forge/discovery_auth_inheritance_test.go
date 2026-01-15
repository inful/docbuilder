package forge

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestDiscoveryService_ConvertToConfigRepositories_InheritsForgeAuth(t *testing.T) {
	forgeCfg := &config.ForgeConfig{
		Name:   "test-gitlab",
		Type:   config.ForgeGitLab,
		Groups: []string{"gitlab-group"},
		Auth: &config.AuthConfig{
			Type:     config.AuthTypeToken,
			Username: "oauth2",
			Token:    "abcd1234token",
		},
	}

	manager, err := CreateForgeManager([]*Config{forgeCfg})
	if err != nil {
		t.Fatalf("CreateForgeManager() error: %v", err)
	}

	// Swap in a deterministic client for this forge.
	gitlab := NewEnhancedGitLabMock(forgeCfg.Name)
	manager.AddForge(forgeCfg, gitlab)

	ds := NewDiscoveryService(manager, &config.FilteringConfig{})
	result, err := ds.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}
	if len(result.Repositories) == 0 {
		t.Fatalf("expected at least one discovered repository")
	}

	cfgRepos := ds.ConvertToConfigRepositories(result.Repositories, manager)
	if len(cfgRepos) == 0 {
		t.Fatalf("expected at least one converted repository")
	}

	for _, r := range cfgRepos {
		if r.Auth == nil {
			t.Fatalf("expected repo auth to be inherited from forge config for %s", r.Name)
		}
		if r.Auth != forgeCfg.Auth {
			t.Fatalf("expected repo auth to reference forge auth; got different pointer")
		}
		if r.Auth.Username != "oauth2" {
			t.Fatalf("expected inherited auth username oauth2, got %q", r.Auth.Username)
		}
	}
}
