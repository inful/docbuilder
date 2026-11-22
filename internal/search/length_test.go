package search

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A long query must not cost the server seconds of work. The index
// builds one term per word, so a query of a megabyte would be a
// megabyte of terms.
func TestSearchCutsALongQuery(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/install/", Title: "Installing", Body: "hello world"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// A query the strict path cannot read, so the forgiving path runs
	// on it: the hyphen is a column name to the index.
	query := strings.Repeat("hello-well ", 50_000)

	start := time.Now()
	results, err := idx.Search(context.Background(), query, 10)
	took := time.Since(start)

	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want none", len(results))
	}
	// The work is done on a short query now, so it is fast.
	if took > 2*time.Second {
		t.Fatalf("the search took %v, want less than 2s", took)
	}
}

// The cut happens on a rune boundary: a letter is not cut in half.
func TestCutQueryKeepsTheLettersWhole(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"a short query stays", "install", "install"},
		{"a long query is cut", strings.Repeat("a", 300), strings.Repeat("a", 256)},
		{"a cut letter is dropped", strings.Repeat("a", 255) + "ö", strings.Repeat("a", 255)},
		{"the space around the cut goes", strings.Repeat("a ", 200), strings.TrimRight(strings.Repeat("a ", 128), " ")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cutQuery(tc.query)
			if got != tc.want {
				t.Fatalf("cutQuery(%d bytes) = %q, want %q", len(tc.query), got, tc.want)
			}
		})
	}
}
