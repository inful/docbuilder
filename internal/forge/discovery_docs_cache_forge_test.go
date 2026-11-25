package forge

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// docsForge is a forge that answers the documentation check with a
// fixed answer.
type docsForge struct {
	Client
	name         string
	hasDocs      bool
	hasDocIgnore bool
	calls        int
}

func (f *docsForge) GetName() string { return f.name }

func (f *docsForge) CheckDocumentation(_ context.Context, repo *Repository) error {
	f.calls++
	repo.HasDocs = f.hasDocs
	repo.HasDocIgnore = f.hasDocIgnore
	return nil
}

// The cache of the documentation check belongs to one forge: two forges
// hold repositories with the same full name, and the answer of one of
// them says nothing about the other one. The marker that keeps a
// repository out of the site must not take the repository of another
// forge out of it.
func TestTheDocumentationCacheIsNotSharedByTheForges(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), &config.FilteringConfig{RequiredPaths: []string{"docs"}})

	first := &docsForge{name: "forge-a", hasDocs: true}
	second := &docsForge{name: "forge-b", hasDocs: false, hasDocIgnore: true}

	moved := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repoA := &Repository{
		FullName:      "acme/docs",
		DefaultBranch: "main",
		LastUpdated:   moved,
		Metadata:      map[string]string{"forge_name": "forge-a"},
	}
	repoB := &Repository{
		FullName:      "acme/docs",
		DefaultBranch: "main",
		LastUpdated:   moved,
		Metadata:      map[string]string{"forge_name": "forge-b"},
	}

	if err := ds.checkDocumentation(context.Background(), first, repoA); err != nil {
		t.Fatalf("check the repository of the first forge: %v", err)
	}
	if err := ds.checkDocumentation(context.Background(), second, repoB); err != nil {
		t.Fatalf("check the repository of the second forge: %v", err)
	}

	if second.calls != 1 {
		t.Fatalf("the second forge was asked %d times, want 1: the answer of the first forge was used", second.calls)
	}
	if !repoB.HasDocIgnore {
		t.Error("the repository of the second forge lost the marker that keeps it out of the site")
	}
	if repoB.HasDocs {
		t.Error("the repository of the second forge has the documentation of the first one")
	}
}
