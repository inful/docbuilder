package search

import (
	"context"
	"testing"
)

// The quoting of a field filter must not change what the query means:
// a wildcard stays a wildcard, a phrase stays a phrase, and a value
// with a colon stays a value.
func TestFieldFilterQuotingKeepsTheMeaning(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"a plain value", "title:kubernetes", "title:kubernetes*"},
		{"a hyphenated value", "categories:how-to", `categories:"how-to"`},
		{"a wildcard", "title:kuber*", `title:"kuber"*`},
		{"a value with a colon", "body:a:b", `body:"a:b"`},
		{"a phrase with a colon", `"1:2"`, `"1:2"`},
		{"a phrase with a hyphen", `"how-to"`, `"how-to"`},
		{"an empty value", "title:", "title:"},
		{"two filters", "title:a body:b", "title:a body:b*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strictMatch(tc.query); got != tc.want {
				t.Fatalf("strictMatch(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

// A wildcard in a field filter still finds the words that start with
// it.
func TestFieldFilterWithAWildcard(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/a/", Title: "Kubernetes cluster", Body: "body"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := idx.Search(context.Background(), "title:kuber*", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}
