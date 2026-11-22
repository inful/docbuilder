package forge

import (
	"context"
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// failingDocsForge fails every documentation check.
type failingDocsForge struct {
	Client
}

func (f *failingDocsForge) GetName() string { return "test" }

func (f *failingDocsForge) CheckDocumentation(_ context.Context, repo *Repository) error {
	repo.HasDocs = false
	return errors.New("the forge is rate limiting us")
}

// A repository whose documentation check failed must stay in the site. The
// forge did not say that the repository has no documentation, and dropping
// it would take its pages out of the site until the next discovery run.
func TestDiscoveryKeepsRepositoryWhenDocsCheckFails(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), &config.FilteringConfig{RequiredPaths: []string{"docs"}})
	client := &failingDocsForge{}

	repo := &Repository{
		Name:          "repo",
		FullName:      "org/repo",
		DefaultBranch: "main",
	}
	if err := ds.checkDocumentation(context.Background(), client, repo); err == nil {
		t.Fatal("expected the check to fail")
	}

	decision := ds.filterDecision(repo)
	if !decision.include {
		t.Fatalf("the repository must be kept, got %q", decision.reason)
	}
}
