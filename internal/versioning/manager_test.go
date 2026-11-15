package versioning

import (
	"testing"
	"time"
)

func newManager() *DefaultVersionManager {
	return NewVersionManager(nil)
}

// The version picker decides which pages exist in the site. A wrong
// pattern drops a version, a wrong path puts it somewhere else, and a
// wrong count deletes documentation. None of it needed a network, so it
// is all tested here.
func TestMatchesPatterns(t *testing.T) {
	vm := newManager()

	tests := []struct {
		name     string
		value    string
		patterns []string
		want     bool
	}{
		{name: "no patterns matches everything", value: "v1.2.3", patterns: nil, want: true},
		{name: "exact match", value: "main", patterns: []string{"main"}, want: true},
		{name: "no match", value: "main", patterns: []string{"release"}, want: false},
		{name: "glob prefix", value: "v1.2.3", patterns: []string{"v*.*.*"}, want: true},
		{name: "glob prefix without match", value: "nightly", patterns: []string{"v*.*.*"}, want: false},
		{name: "one of many patterns", value: "develop", patterns: []string{"nope", "develop"}, want: true},
		{name: "invalid pattern is skipped", value: "main", patterns: []string{"[", "main"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vm.matchesPatterns(tt.value, tt.patterns); got != tt.want {
				t.Errorf("matchesPatterns(%q, %v) = %v, want %v", tt.value, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestGenerateVersionPath(t *testing.T) {
	vm := newManager()

	tests := []struct {
		name          string
		refName       string
		refType       string
		defaultBranch string
		want          string
	}{
		{name: "default branch", refName: "main", refType: "branch", defaultBranch: "main", want: "latest"},
		{name: "other branch", refName: "develop", refType: "branch", defaultBranch: "main", want: "develop"},
		{name: "tag", refName: "v1.2.3", refType: "tag", defaultBranch: "main", want: "v1.2.3"},
		{name: "uppercase is lowered", refName: "Release", refType: "branch", defaultBranch: "main", want: "release"},
		{name: "dashes are kept", refName: "release-1.2", refType: "branch", defaultBranch: "main", want: "release-1.2"},
		{name: "slashes become dashes", refName: "feature/new", refType: "branch", defaultBranch: "main", want: "feature-new"},
		{name: "underscores become dashes", refName: "my_branch", refType: "branch", defaultBranch: "main", want: "my-branch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vm.generateVersionPath(tt.refName, tt.refType, tt.defaultBranch); got != tt.want {
				t.Errorf("generateVersionPath(%q, %q, %q) = %q, want %q",
					tt.refName, tt.refType, tt.defaultBranch, got, tt.want)
			}
		})
	}
}

func TestGenerateDisplayName(t *testing.T) {
	vm := newManager()

	tests := []struct {
		name    string
		refName string
		refType string
		want    string
	}{
		{name: "main is latest", refName: "main", refType: "branch", want: "Latest"},
		{name: "master is latest", refName: "master", refType: "branch", want: "Latest"},
		{name: "develop", refName: "develop", refType: "branch", want: "Development"},
		{name: "other branch is capitalized", refName: "feature-x", refType: "branch", want: "Feature x"},
		{name: "tag is kept as is", refName: "v1.2.3", refType: "tag", want: "v1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vm.generateDisplayName(tt.refName, tt.refType); got != tt.want {
				t.Errorf("generateDisplayName(%q, %q) = %q, want %q", tt.refName, tt.refType, got, tt.want)
			}
		})
	}
}

func TestFilterAndConvertReferences(t *testing.T) {
	vm := newManager()

	refs := []*GitReference{
		{Name: "main", Type: VersionTypeBranch, CommitSHA: "aaa"},
		{Name: "develop", Type: VersionTypeBranch, CommitSHA: "bbb"},
		{Name: "feature/x", Type: VersionTypeBranch, CommitSHA: "ccc"},
		{Name: "v1.0.0", Type: VersionTypeTag, CommitSHA: "ddd"},
		{Name: "nightly", Type: VersionTypeTag, CommitSHA: "eee"},
	}

	tests := []struct {
		name       string
		strategy   VersionStrategy
		branchPat  []string
		tagPat     []string
		wantNames  []string
		wantIsDflt string
	}{
		{
			name:      "default only",
			strategy:  StrategyDefaultOnly,
			wantNames: []string{"main"},
		},
		{
			name:      "branches",
			strategy:  StrategyBranches,
			branchPat: []string{"main", "develop"},
			wantNames: []string{"main", "develop"},
		},
		{
			name:      "tags",
			strategy:  StrategyTags,
			tagPat:    []string{"v*.*.*"},
			wantNames: []string{"v1.0.0"},
		},
		{
			name:      "branches and tags",
			strategy:  StrategyBranchesAndTags,
			branchPat: []string{"main"},
			tagPat:    []string{"v*.*.*"},
			wantNames: []string{"main", "v1.0.0"},
		},
		{
			name:      "no pattern matches nothing",
			strategy:  StrategyBranches,
			branchPat: []string{"release"},
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &VersionConfig{
				Strategy:       tt.strategy,
				BranchPatterns: tt.branchPat,
				TagPatterns:    tt.tagPat,
			}
			versions := vm.filterAndConvertReferences(refs, cfg, "main")

			if len(versions) != len(tt.wantNames) {
				t.Fatalf("got %d versions (%v), want %d", len(versions), versionNames(versions), len(tt.wantNames))
			}
			for i, want := range tt.wantNames {
				if versions[i].Name != want {
					t.Errorf("version %d = %q, want %q", i, versions[i].Name, want)
				}
			}
		})
	}
}

// The default branch is the one version that is not versioned: it lives
// at "latest" and is the one the reader sees first.
func TestFilterAndConvertReferences_MarksTheDefaultBranch(t *testing.T) {
	vm := newManager()
	cfg := &VersionConfig{
		Strategy:       StrategyBranchesAndTags,
		BranchPatterns: []string{"main", "develop"},
		TagPatterns:    []string{"v*.*.*"},
	}
	refs := []*GitReference{
		{Name: "main", Type: VersionTypeBranch, CommitSHA: "aaa"},
		{Name: "develop", Type: VersionTypeBranch, CommitSHA: "bbb"},
		{Name: "v1.0.0", Type: VersionTypeTag, CommitSHA: "ccc"},
	}

	versions := vm.filterAndConvertReferences(refs, cfg, "main")
	if len(versions) != 3 {
		t.Fatalf("got %d versions, want 3", len(versions))
	}

	var defaults []string
	for _, v := range versions {
		if v.IsDefault {
			defaults = append(defaults, v.Name)
		}
	}
	if len(defaults) != 1 || defaults[0] != "main" {
		t.Fatalf("default versions = %v, want [main]", defaults)
	}
	if versions[0].Path != "latest" {
		t.Errorf("default branch path = %q, want \"latest\"", versions[0].Path)
	}
}

func TestCalculateVersionChanges(t *testing.T) {
	vm := newManager()

	old := []*Version{
		{Name: "main", CommitSHA: "aaa"},
		{Name: "v1.0.0", CommitSHA: "bbb"},
	}
	newer := []*Version{
		{Name: "main", CommitSHA: "aaa"},
		{Name: "v1.0.0", CommitSHA: "ccc"},
		{Name: "v2.0.0", CommitSHA: "ddd"},
	}

	newCount, updatedCount, removedCount := vm.calculateVersionChanges(old, newer)
	if newCount != 1 {
		t.Errorf("newCount = %d, want 1", newCount)
	}
	if updatedCount != 1 {
		t.Errorf("updatedCount = %d, want 1", updatedCount)
	}
	if removedCount != 0 {
		t.Errorf("removedCount = %d, want 0", removedCount)
	}
}

// A version that disappeared is counted as removed, so the build knows a
// page went away.
func TestCalculateVersionChanges_CountsRemoved(t *testing.T) {
	vm := newManager()

	old := []*Version{
		{Name: "main", CommitSHA: "aaa"},
		{Name: "v1.0.0", CommitSHA: "bbb"},
	}
	newer := []*Version{
		{Name: "main", CommitSHA: "aaa"},
	}

	newCount, updatedCount, removedCount := vm.calculateVersionChanges(old, newer)
	if newCount != 0 || updatedCount != 0 || removedCount != 1 {
		t.Errorf("new=%d updated=%d removed=%d, want 0/0/1", newCount, updatedCount, removedCount)
	}
}

func versionNames(versions []*Version) []string {
	names := make([]string, len(versions))
	for i, v := range versions {
		names[i] = v.Name
	}
	return names
}

// The version carries the commit it was built from, so a reader can see
// what they are looking at.
func TestConvertReferenceToVersion_KeepsTheCommit(t *testing.T) {
	vm := newManager()
	cfg := &VersionConfig{Strategy: StrategyDefaultOnly}

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ref := &GitReference{Name: "main", Type: VersionTypeBranch, CommitSHA: "abc123", CreatedAt: created}

	version := vm.convertReferenceToVersion(ref, cfg, "main")
	if version == nil {
		t.Fatal("expected a version, got nil")
	}
	if version.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %q, want \"abc123\"", version.CommitSHA)
	}
	if !version.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", version.CreatedAt, created)
	}
}
