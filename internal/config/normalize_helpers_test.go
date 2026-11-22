package config

import (
	"testing"
)

// The helper canonicalises a configured list: it trims, drops the
// empty entries, removes the duplicates and sorts what is left. A
// wrong answer changes what the daemon looks for.
func TestNormalizeStringSlice(t *testing.T) {
	tests := []struct {
		name        string
		in          []string
		want        []string
		wantWarning bool
	}{
		{name: "an empty list", in: nil, want: nil},
		{name: "one entry", in: []string{"docs"}, want: []string{"docs"}},
		{name: "the list is sorted", in: []string{"b", "a", "c"}, want: []string{"a", "b", "c"}},
		{name: "the entries are trimmed", in: []string{"  docs  "}, want: []string{"docs"}, wantWarning: true},
		{name: "the empty entries are dropped", in: []string{"docs", "  "}, want: []string{"docs"}, wantWarning: true},
		{name: "the duplicates are dropped", in: []string{"docs", "docs"}, want: []string{"docs"}, wantWarning: true},
		{name: "the duplicate is found after trimming", in: []string{"docs", " docs "}, want: []string{"docs"}, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &NormalizationResult{}
			got := normalizeStringSlice("test", tt.in, res)

			if len(got) != len(tt.want) {
				t.Fatalf("normalizeStringSlice(%q) = %q, want %q", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("normalizeStringSlice(%q) = %q, want %q", tt.in, got, tt.want)
				}
			}

			if tt.wantWarning && len(res.Warnings) == 0 {
				t.Error("expected a warning about the changed list")
			}
			if !tt.wantWarning && len(res.Warnings) != 0 {
				t.Errorf("expected no warning, got %q", res.Warnings)
			}
		})
	}
}
