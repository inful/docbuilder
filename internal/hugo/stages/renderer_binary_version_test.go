package stages

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/pipeline"
)

// The Hugo of the operator has to be new enough for the theme.
//
// The theme asks the site for the direction of its language, and Hugo
// added that field in v0.158.0. An older Hugo fails on the first page
// with "can't evaluate field Direction in type *langs.Language", and
// the build says nothing about the version that is too old.
func TestTheHugoOfTheOperatorIsCheckedAgainstTheTheme(t *testing.T) {
	cases := []struct {
		version string
		tooOld  bool
	}{
		{version: "0.157.0", tooOld: true},
		{version: "0.150.2", tooOld: true},
		{version: "0.157.9", tooOld: true},
		{version: pipeline.MinHugoVersion, tooOld: false},
		{version: "0.158.0", tooOld: false},
		{version: "0.168.0", tooOld: false},
		{version: "1.0.0", tooOld: false},
		{version: "", tooOld: false},
	}

	for _, tc := range cases {
		if got := hugoVersionTooOld(tc.version); got != tc.tooOld {
			t.Errorf("hugoVersionTooOld(%q) = %v, want %v", tc.version, got, tc.tooOld)
		}
	}
}

// The comparison of the versions is a comparison of the numbers.
func TestTheVersionsAreComparedByTheirNumbers(t *testing.T) {
	if compareVersions("0.158.0", "0.158.0") != 0 {
		t.Error("the versions are not equal")
	}
	if compareVersions("0.157.9", "0.158.0") >= 0 {
		t.Error("the older version is not older")
	}
	if compareVersions("0.159.0", "0.158.0") <= 0 {
		t.Error("the newer version is not newer")
	}
}
