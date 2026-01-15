package forge

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestDiscoveryService_FilterDecision(t *testing.T) {
	t.Parallel()

	cfg := &config.FilteringConfig{
		RequiredPaths:   []string{"docs"},
		IncludePatterns: []string{"inc-*"},
		ExcludePatterns: []string{"*blocked*"},
	}
	ds := &DiscoveryService{filtering: cfg}

	cases := []struct {
		name        string
		repo        *Repository
		wantInclude bool
		wantReason  string
		wantDetail  string
	}{
		{
			name:        "archived",
			repo:        &Repository{Name: "inc-one", FullName: "g/inc-one", Archived: true, HasDocs: true},
			wantInclude: false,
			wantReason:  "archived",
		},
		{
			name:        "docignore present",
			repo:        &Repository{Name: "inc-one", FullName: "g/inc-one", HasDocIgnore: true, HasDocs: true},
			wantInclude: false,
			wantReason:  "docignore_present",
		},
		{
			name:        "missing required paths",
			repo:        &Repository{Name: "inc-one", FullName: "g/inc-one", HasDocs: false},
			wantInclude: false,
			wantReason:  "missing_required_paths",
		},
		{
			name:        "include patterns miss",
			repo:        &Repository{Name: "other", FullName: "g/other", HasDocs: true},
			wantInclude: false,
			wantReason:  "include_patterns_miss",
		},
		{
			name:        "exclude patterns match",
			repo:        &Repository{Name: "inc-blocked", FullName: "g/inc-blocked", HasDocs: true},
			wantInclude: false,
			wantReason:  "exclude_patterns_match",
			wantDetail:  "*blocked*",
		},
		{
			name:        "included",
			repo:        &Repository{Name: "inc-ok", FullName: "g/inc-ok", HasDocs: true},
			wantInclude: true,
			wantReason:  "included",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ds.filterDecision(tc.repo)
			if got.include != tc.wantInclude {
				t.Fatalf("include: got %v want %v", got.include, tc.wantInclude)
			}
			if got.reason != tc.wantReason {
				t.Fatalf("reason: got %q want %q", got.reason, tc.wantReason)
			}
			if got.detail != tc.wantDetail {
				t.Fatalf("detail: got %q want %q", got.detail, tc.wantDetail)
			}
		})
	}
}
