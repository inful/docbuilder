package hugo

import (
    "errors"
    "testing"
    gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
)

func TestClassifyGitFailureTyped(t *testing.T) {
    cases := []struct {
        name string
        err  error
        want ReportIssueCode
    }{
        {"auth", &gitpkg.AuthError{Op: "clone", URL: "u", Err: errors.New("auth")}, IssueAuthFailure},
        {"notfound", &gitpkg.NotFoundError{Op: "clone", URL: "u", Err: errors.New("not found")}, IssueRepoNotFound},
        {"unsupported", &gitpkg.UnsupportedProtocolError{Op: "clone", URL: "u", Err: errors.New("unsupported protocol")}, IssueUnsupportedProto},
        {"diverged", &gitpkg.RemoteDivergedError{Op: "update", URL: "u", Branch: "main", Err: errors.New("diverged branch")}, IssueRemoteDiverged},
        {"ratelimit", &gitpkg.RateLimitError{Op: "clone", URL: "u", Err: errors.New("rate limit exceeded")}, IssueRateLimit},
        {"timeout", &gitpkg.NetworkTimeoutError{Op: "clone", URL: "u", Err: errors.New("network timeout")}, IssueNetworkTimeout},
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
        want ReportIssueCode
    }{
        {"authentication failed for remote", IssueAuthFailure},
        {"repository not found on server", IssueRepoNotFound},
        {"unsupported protocol scheme xyz", IssueUnsupportedProto},
        {"local branch diverged and hard reset disabled", IssueRemoteDiverged},
        {"some random error", ""},
        {"request failed due to rate limit", IssueRateLimit},
        {"operation i/o timeout while reading", IssueNetworkTimeout},
    }
    for _, c := range cases {
        got := classifyGitFailure(errors.New(c.msg))
        if got != c.want {
            t.Fatalf("heuristic for %q expected %s got %s", c.msg, c.want, got)
        }
    }
}
