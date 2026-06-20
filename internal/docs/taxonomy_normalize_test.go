package docs

import (
	"reflect"
	"testing"
)

func TestNormalizeTaxonomyValues(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil input returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty input returns nil",
			in:   []string{},
			want: nil,
		},
		{
			name: "whitespace-only entries are dropped (returns empty, not nil)",
			in:   []string{"", "   ", "\t"},
			want: []string{},
		},
		{
			name: "values are trimmed",
			in:   []string{"  alpha  ", "Beta"},
			want: []string{"alpha", "Beta"},
		},
		{
			name: "case-insensitive dedup preserves first-seen casing",
			in:   []string{"alpha", "Beta", "ALPHA", "beta"},
			want: []string{"alpha", "Beta"},
		},
		{
			name: "result is sorted case-insensitively",
			in:   []string{"Charlie", "alpha", "Beta"},
			want: []string{"alpha", "Beta", "Charlie"},
		},
		{
			name: "sort is case-insensitive; first-seen wins dedup",
			in:   []string{"b", "A", "B", "a"},
			want: []string{"A", "b"}, // dedup keeps "b" then "A"; sort reorders by lower-case
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeTaxonomyValues(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeTaxonomyValues(%#v) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}
