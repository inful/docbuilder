package search

import (
	"context"
	"testing"
)

// A field filter must mean what the reader wrote. The index reads a
// hyphen as the end of the field name, so a value with one has to be
// quoted.
func TestFieldFilterWithAHyphenatedValue(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/install/", Title: "Installing", Categories: "how-to", Tags: "how-to", Body: "body"},
		{URL: "/other/", Title: "Other", Categories: "reference", Tags: "other", Body: "body"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ctx := context.Background()

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"a hyphenated value", "categories:how-to", "/install/"},
		{"a hyphenated tag", "tags:how-to", "/install/"},
		{"a plain value", "categories:reference", "/other/"},
		{"a quoted value", `categories:"how-to"`, "/install/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := idx.Search(ctx, tc.query, 10)
			if err != nil {
				t.Fatalf("search %q: %v", tc.query, err)
			}
			if len(results) != 1 || results[0].URL != tc.want {
				got := make([]string, 0, len(results))
				for _, r := range results {
					got = append(got, r.URL)
				}
				t.Fatalf("search %q returned %v, want %q", tc.query, got, tc.want)
			}
		})
	}
}

// A field filter that names a value nobody has finds nothing.
func TestFieldFilterWithAnUnknownValue(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/install/", Title: "Installing", Categories: "how-to", Body: "body"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := idx.Search(context.Background(), "categories:reference", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want none", len(results))
	}
}

// The quoting must not break the queries that already worked.
func TestFieldFilterQuotingKeepsTheOtherQueries(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"a plain filter", "title:kubernetes", `title:kubernetes*`},
		{"a quoted filter", `title:"package manager"`, `title:"package manager"`},
		{"a filter with a phrase", `body:"package manager"`, `body:"package manager"`},
		{"no filter at all", "kubernetes", "kubernetes*"},
		{"a filter in the middle", "kubernetes title:deploy", "kubernetes title:deploy*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strictMatch(tc.query); got != tc.want {
				t.Fatalf("strictMatch(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}
