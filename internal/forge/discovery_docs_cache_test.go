package forge

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// countingForge counts the CheckDocumentation calls it receives.
type countingForge struct {
	Client
	name  string
	calls int
}

func (c *countingForge) GetName() string { return c.name }

func (c *countingForge) CheckDocumentation(_ context.Context, repo *Repository) error {
	c.calls++
	repo.HasDocs = true
	repo.HasDocIgnore = false
	return nil
}

// The documentation check costs two API calls per repository and the
// answer only changes when the repository changes. A repository that has
// not moved must not be asked again.
func TestCheckDocumentation_CachesUnchangedRepositories(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), &config.FilteringConfig{RequiredPaths: []string{"docs"}})
	client := &countingForge{name: "test"}

	repo := &Repository{
		FullName:      "org/repo",
		DefaultBranch: "main",
		LastUpdated:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := ds.checkDocumentation(context.Background(), client, repo); err != nil {
		t.Fatalf("first check: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("calls after the first check = %d, want 1", client.calls)
	}
	if !repo.HasDocs {
		t.Fatal("HasDocs = false, want true")
	}

	// Same repository, same timestamp: the answer comes from the cache.
	second := &Repository{
		FullName:      "org/repo",
		DefaultBranch: "main",
		LastUpdated:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := ds.checkDocumentation(context.Background(), client, second); err != nil {
		t.Fatalf("second check: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("calls after the second check = %d, want 1 (the repository did not change)", client.calls)
	}
	if !second.HasDocs {
		t.Fatal("cached HasDocs = false, want true")
	}

	// The repository moved: the answer is asked again.
	moved := &Repository{
		FullName:      "org/repo",
		DefaultBranch: "main",
		LastUpdated:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	if err := ds.checkDocumentation(context.Background(), client, moved); err != nil {
		t.Fatalf("third check: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("calls after the repository moved = %d, want 2", client.calls)
	}
}

// The cache holds one entry per repository. A repository that moved many
// times must not leave one entry per move behind, or the map grows for the
// life of the daemon.
func TestCheckDocumentation_CacheStaysBounded(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), &config.FilteringConfig{RequiredPaths: []string{"docs"}})
	client := &countingForge{name: "test"}

	for day := range 30 {
		repo := &Repository{
			FullName:      "org/repo",
			DefaultBranch: "main",
			LastUpdated:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, day),
		}
		if err := ds.checkDocumentation(context.Background(), client, repo); err != nil {
			t.Fatalf("check %d: %v", day, err)
		}
	}

	if client.calls != 30 {
		t.Fatalf("calls = %d, want 30", client.calls)
	}
	if len(ds.docsCache) != 1 {
		t.Fatalf("cache entries = %d, want 1", len(ds.docsCache))
	}
}

// A repository without a last-updated timestamp cannot be cached: the
// forge did not say when it last changed, so the answer may be stale.
func TestCheckDocumentation_DoesNotCacheWithoutTimestamp(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), &config.FilteringConfig{RequiredPaths: []string{"docs"}})
	client := &countingForge{name: "test"}

	for range 2 {
		repo := &Repository{FullName: "org/repo", DefaultBranch: "main"}
		if err := ds.checkDocumentation(context.Background(), client, repo); err != nil {
			t.Fatalf("check: %v", err)
		}
	}

	if client.calls != 2 {
		t.Fatalf("calls = %d, want 2 (no timestamp, no cache)", client.calls)
	}
}
