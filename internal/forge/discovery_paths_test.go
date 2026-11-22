package forge

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The paths that hold the documentation are the ones the operator
// configured. The forge is asked about those paths, and the repository
// keeps them.
func TestTheConfiguredPathsAreUsed(t *testing.T) {
	service := NewDiscoveryService(NewForgeManager(), &config.FilteringConfig{
		RequiredPaths: []string{"documentation"},
	})

	repo := &Repository{ID: "1", FullName: "org/repo", Name: "repo"}
	client := &countingForge{name: "mock"}

	if err := service.checkDocumentation(context.Background(), client, repo); err != nil {
		t.Fatalf("check the documentation: %v", err)
	}

	if len(repo.RequiredPaths) != 1 || repo.RequiredPaths[0] != "documentation" {
		t.Fatalf("the paths of the repository are %v", repo.RequiredPaths)
	}
}

// A repository that is not asked about any path keeps the default.
func TestTheDefaultPathsAreUsedWithoutAConfiguration(t *testing.T) {
	service := NewDiscoveryService(NewForgeManager(), nil)

	repo := &Repository{ID: "1", FullName: "org/repo", Name: "repo"}
	client := &countingForge{name: "mock"}

	if err := service.checkDocumentation(context.Background(), client, repo); err != nil {
		t.Fatalf("check the documentation: %v", err)
	}

	if len(repo.RequiredPaths) != 1 || repo.RequiredPaths[0] != "docs" {
		t.Fatalf("the paths of the repository are %v", repo.RequiredPaths)
	}
}

// The paths of a repository reach the configuration of the build.
func TestThePathsOfTheRepositoryReachTheConfiguration(t *testing.T) {
	repo := &Repository{
		ID:            "1",
		FullName:      "org/repo",
		Name:          "repo",
		CloneURL:      "https://example.com/org/repo.git",
		RequiredPaths: []string{"documentation"},
	}

	converted := repo.ToConfigRepository(nil)
	if len(converted.Paths) != 1 || converted.Paths[0] != "documentation" {
		t.Fatalf("the paths of the repository are %v", converted.Paths)
	}
}
