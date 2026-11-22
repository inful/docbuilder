package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// openTestIndex opens an index in a temporary directory.
func openTestIndex(t *testing.T) *Index {
	t.Helper()
	idx, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	return idx
}

// sampleDocs is a small corpus for the tests.
func sampleDocs() []Document {
	return []Document{
		{URL: "/install/", Title: "Installing the daemon", Categories: "how-to", Tags: "install linux", Body: "Install the daemon with the package manager on linux."},
		{URL: "/deploy/kubernetes/", Title: "Deploying to Kubernetes", Categories: "how-to", Tags: "deploy", Body: "Deploy the daemon to a kubernetes cluster with a helm chart."},
		{URL: "/adr/adr-001/", Title: "ADR-001: Staging", Categories: "architecture-decisions", Tags: "adr", Body: "The build renders into a staging directory and promotes it."},
	}
}

// The index must find a page by a word in its body, by its title and by
// its tags.
func TestSearchFindsPages(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"a word in the body", "package manager", "/install/"},
		{"a word in the title", "kubernetes", "/deploy/kubernetes/"},
		{"a word in the tags", "helm", "/deploy/kubernetes/"},
		{"a stem", "installing", "/install/"},
		{"a prefix", "kuber", "/deploy/kubernetes/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := idx.Search(context.Background(), tc.query, 10)
			if err != nil {
				t.Fatalf("search %q: %v", tc.query, err)
			}
			if len(results) == 0 {
				t.Fatalf("search %q returned nothing", tc.query)
			}
			if results[0].URL != tc.want {
				t.Fatalf("search %q returned %q first, want %q", tc.query, results[0].URL, tc.want)
			}
		})
	}
}

// A title hit must beat a body hit.
func TestSearchRanksTitleHitsFirst(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/body/", Title: "Something else", Body: "The daemon talks about staging and promotion."},
		{URL: "/title/", Title: "Staging and promotion", Body: "Nothing to see here."},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := idx.Search(context.Background(), "staging", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected two hits, got %d", len(results))
	}
	if results[0].URL != "/title/" {
		t.Fatalf("first hit = %q, want the title hit", results[0].URL)
	}
}

// The index must hold exactly the pages of the last build: a page that
// is gone from the site must be gone from the index.
func TestUpsertReplacesTheWholeIndex(t *testing.T) {
	idx := openTestIndex(t)
	ctx := context.Background()

	if err := idx.Upsert(ctx, sampleDocs()); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := idx.Upsert(ctx, []Document{{URL: "/new/", Title: "New", Body: "The new page."}}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	count, err := idx.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("the index holds %d pages, want 1", count)
	}
	results, err := idx.Search(ctx, "kubernetes", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("the old page is still in the index: %+v", results)
	}
}

// A query that is not a word must not break the search.
func TestSearchIgnoresPunctuation(t *testing.T) {
	idx := openTestIndex(t)
	ctx := context.Background()
	if err := idx.Upsert(ctx, sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	for _, query := range []string{"", "   ", "  ???  ", `he"llo`, "a AND b", "(", "*"} {
		if _, err := idx.Search(ctx, query, 10); err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
	}
}

// The excerpt must show where the page matched.
func TestSearchReturnsAnExcerpt(t *testing.T) {
	idx := openTestIndex(t)
	ctx := context.Background()
	if err := idx.Upsert(ctx, sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := idx.Search(ctx, "package manager", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a hit")
	}
	if results[0].Excerpt == "" {
		t.Fatal("expected an excerpt")
	}
	if results[0].Title != "Installing the daemon" {
		t.Fatalf("title = %q", results[0].Title)
	}
}

// The index must survive a restart: the daemon opens it again after a
// restart.
func TestIndexSurvivesAReopen(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	idx, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := idx.Upsert(ctx, sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := idx.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	count, err := reopened.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != len(sampleDocs()) {
		t.Fatalf("the reopened index holds %d pages, want %d", count, len(sampleDocs()))
	}
	if _, err := os.Stat(filepath.Join(dir, "search.db")); err != nil {
		t.Fatalf("the index file must exist: %v", err)
	}
}
