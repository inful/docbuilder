package hugo

import (
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// A build that did not publish is a failed build.
//
// The derivation of the outcome looks at the errors and the warnings: a
// clone failure that the classification does not recognize is only a
// warning, and the report of the build said warning although a
// repository was missing from the site that was published.
func TestABuildThatDidNotPublishIsAFailedBuild(t *testing.T) {
	report := &models.BuildReport{
		Warnings:           []error{errors.New("the clone of a repository failed")},
		FailedRepositories: 1,
	}

	failedOutcome(report)

	if report.Outcome != models.OutcomeFailed {
		t.Fatalf("the outcome of the build is %q, want %q", report.Outcome, models.OutcomeFailed)
	}
}
