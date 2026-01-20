package stages

import (
	"errors"
	"testing"

	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func TestClassifyGitFailureTyped(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want models.ReportIssueCode
	}{
		{"auth", gitpkg.ClassifyGitError(errors.New("authentication failed"), "clone", "u"), models.IssueAuthFailure},
		{"notfound", gitpkg.ClassifyGitError(errors.New("repository not found"), "clone", "u"), models.IssueRepoNotFound},
		{"unsupported", gitpkg.ClassifyGitError(errors.New("unsupported protocol"), "clone", "u"), models.IssueUnsupportedProto},
		{"diverged", gitpkg.ClassifyGitError(errors.New("local branch diverged"), "update", "u"), models.IssueRemoteDiverged},
		{"ratelimit", gitpkg.ClassifyGitError(errors.New("rate limit exceeded"), "clone", "u"), models.IssueRateLimit},
		{"timeout", gitpkg.ClassifyGitError(errors.New("network timeout"), "clone", "u"), models.IssueNetworkTimeout},
	}
	for _, c := range cases {
		if got := classifyGitFailure(c.err); got != c.want {
			t.Fatalf("%s: expected %s got %s", c.name, c.want, got)
		}
	}
}

func TestClassifyGitFailureHeuristic(t *testing.T) {
	cases := []struct {
		msg  string
		want models.ReportIssueCode
	}{
		{"authentication failed for remote", models.IssueAuthFailure},
		{"repository not found on server", models.IssueRepoNotFound},
		{"unsupported protocol scheme xyz", models.IssueUnsupportedProto},
		{"local branch diverged and hard reset disabled", models.IssueRemoteDiverged},
		{"some random error", ""},
		{"request failed due to rate limit", models.IssueRateLimit},
		{"operation i/o timeout while reading", models.IssueNetworkTimeout},
	}
	for _, c := range cases {
		got := classifyGitFailure(errors.New(c.msg))
		if got != c.want {
			t.Fatalf("heuristic for %q expected %s got %s", c.msg, c.want, got)
		}
	}
}
