package stages

import (
	"strings"
	"testing"
)

// The failure of Hugo carries what Hugo said.
//
// The operator saw "exit status 1" and the reason was only in the log
// of the daemon.
func TestTheFailureOfHugoCarriesWhatHugoSaid(t *testing.T) {
	stderr := strings.Join([]string{
		"INFO  build:  step process substep collect files 98 files_total",
		"ERROR render of \"/docs/guide.md\" failed: error calling transform: nil pointer evaluating",
		"ERROR site failed to render",
	}, "\n")

	summary := hugoFailureSummary("", stderr)

	if !strings.Contains(summary, "nil pointer evaluating") {
		t.Fatalf("the summary does not carry the error of Hugo: %q", summary)
	}
	if strings.Contains(summary, "step process") {
		t.Fatalf("the summary carries the progress of the build: %q", summary)
	}
}

// A failure without an error line carries the last lines of the output.
func TestTheFailureOfHugoWithoutAnErrorLineCarriesTheOutput(t *testing.T) {
	stderr := "INFO  build:  step render pages 383 content\nERROR something went wrong"

	summary := hugoFailureSummary("", stderr)

	if !strings.Contains(summary, "something went wrong") {
		t.Fatalf("the summary does not carry the output: %q", summary)
	}
}
