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
		{"auth", &gitpkg.AuthError{Op: "clone", URL: "u", Err: errors.New("auth")}, models.IssueAuthFailure},
		{"notfound", &gitpkg.NotFoundError{Op: "clone", URL: "u", Err: errors.New("not found")}, models.IssueRepoNotFound},
		{"unsupported", &gitpkg.UnsupportedProtocolError{Op: "clone", URL: "u", Err: errors.New("unsupported protocol")}, models.IssueUnsupportedProto},
		{"diverged", &gitpkg.RemoteDivergedError{Op: "update", URL: "u", Branch: "main", Err: errors.New("diverged branch")}, models.IssueRemoteDiverged},
		{"ratelimit", &gitpkg.RateLimitError{Op: "clone", URL: "u", Err: errors.New("rate limit exceeded")}, models.IssueRateLimit},
		{"timeout", &gitpkg.NetworkTimeoutError{Op: "clone", URL: "u", Err: errors.New("network timeout")}, models.IssueNetworkTimeout},
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
