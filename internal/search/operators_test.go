package search

import (
	"context"
	"slices"
	"sort"
	"testing"
)

// The operators of the query language must work: a reader who writes
// them means them.
func TestSearchWithOperators(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/install/", Title: "Installing the daemon", Tags: "install linux", Body: "Install the daemon with the package manager on linux."},
		{URL: "/deploy/", Title: "Deploying to Kubernetes", Tags: "deploy", Body: "Deploy the daemon to a kubernetes cluster with a helm chart."},
		{URL: "/adr/", Title: "ADR-001 Staging", Tags: "adr", Body: "The build renders into a staging directory and promotes it."},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ctx := context.Background()

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"AND needs both terms", "daemon AND kubernetes", []string{"/deploy/"}},
		{"OR takes either term", "kubernetes OR staging", []string{"/deploy/", "/adr/"}},
		{"NOT leaves a term out", "daemon NOT kubernetes", []string{"/install/"}},
		{"a phrase has to appear as it is written", `"package manager"`, []string{"/install/"}},
		{"a phrase that is not written as it is", `"manager package"`, nil},
		{"a field filter holds the term in that field", "title:kubernetes", []string{"/deploy/"}},
		{"a field filter keeps the term out of the other fields", "title:helm", nil},
		{"a field filter on the body", "body:helm", []string{"/deploy/"}},
		{"a field filter on the tags", "tags:adr", []string{"/adr/"}},
		{"a prefix on the last term", "kubernetes OR kuber*", []string{"/deploy/"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := idx.Search(ctx, tc.query, 10)
			if err != nil {
				t.Fatalf("search %q: %v", tc.query, err)
			}
			got := make([]string, 0, len(results))
			for _, r := range results {
				got = append(got, r.URL)
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !slices.Equal(got, want) {
				t.Fatalf("search %q returned %v, want %v", tc.query, got, want)
			}
		})
	}
}

// A query the index cannot read must still find the pages it can: a
// search box holds half-written queries.
func TestSearchFallsBackToTheForgivingQuery(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/install/", Title: "Installing the daemon", Body: "Install the daemon with the package manager."},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ctx := context.Background()

	// A query the index cannot read: the operator at the end is not
	// finished, and a word with a hyphen is not a term the index knows.
	for _, query := range []string{"daemon AND", "daemon OR", "package-manager"} {
		results, err := idx.Search(ctx, query, 10)
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		if len(results) == 0 {
			t.Fatalf("search %q returned nothing", query)
		}
	}
}

// A query that holds no word at all finds nothing, and it is not an
// error.
func TestSearchWithNoWordFindsNothing(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	for _, query := range []string{"???", "AND", "  ", `"`} {
		results, err := idx.Search(context.Background(), query, 10)
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		if len(results) != 0 {
			t.Fatalf("search %q returned %d results, want none", query, len(results))
		}
	}
}

// The last term of a query may be a prefix, so that the reader finds
// the pages while they are still typing.
func TestStrictMatch(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"a word", "kube", "kube*"},
		{"the last of two words", "deploy kube", "deploy kube*"},
		{"the last term after an operator", "daemon OR kube", "daemon OR kube*"},
		{"the last term after a field filter", "title:kube", "title:kube*"},
		{"a word that already is a prefix", "kube*", "kube*"},
		{"a phrase keeps its words", `"package manager"`, `"package manager"`},
		{"a phrase with a field filter", `title:"package manager"`, `title:"package manager"`},
		{"an operator at the end is not a term", "daemon OR", "daemon OR"},
		{"a group at the end", "(daemon OR kube)", "(daemon OR kube)"},
		{"nothing to change", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strictMatch(tc.query); got != tc.want {
				t.Fatalf("strictMatch(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}
