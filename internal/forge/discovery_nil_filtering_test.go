package forge

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// filteringForge is a forge with one repository that holds no
// documentation: the repository is filtered out of the site.
type filteringForge struct {
	Client
	name string
}

func (f *filteringForge) GetName() string { return f.name }

func (f *filteringForge) GetType() Type { return TypeGitLab }

func (f *filteringForge) ListOrganizations(context.Context) ([]*Organization, error) {
	return nil, nil
}

func (f *filteringForge) ListRepositories(context.Context, []string) ([]*Repository, error) {
	return []*Repository{{
		ID:            "1",
		Name:          "docs",
		FullName:      "acme/docs",
		CloneURL:      "https://gitlab.example.com/acme/docs.git",
		DefaultBranch: "main",
	}}, nil
}

func (f *filteringForge) CheckDocumentation(_ context.Context, repo *Repository) error {
	repo.HasDocs = false
	repo.HasDocIgnore = false
	return nil
}

// A discovery service without a filtering configuration must not take
// the process down: the rest of the package copes with the missing
// configuration, and the warning about the filtered repositories read
// it without a check.
func TestTheDiscoveryWithoutAFilteringConfigurationDoesNotPanic(t *testing.T) {
	forgeCfg := &config.ForgeConfig{
		Name: "test-gitlab",
		Type: config.ForgeGitLab,
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
	manager.AddForge(forgeCfg, &filteringForge{name: forgeCfg.Name})

	ds := NewDiscoveryService(manager, nil)

	result, err := ds.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}
	if len(result.Repositories) != 0 {
		t.Fatalf("the repository without documentation is in the site: %d repositories", len(result.Repositories))
	}
}
