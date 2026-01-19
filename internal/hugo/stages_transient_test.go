package hugo

import (
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
)

func TestStageErrorTransient(t *testing.T) {
	cases := []struct {
		stage models.StageName
		err   error
		kind  models.StageErrorKind
		want  bool
	}{
		{models.StageCloneRepos, build.ErrClone, models.StageErrorWarning, true},
		{models.StageRunHugo, build.ErrHugo, models.StageErrorWarning, true},
		{models.StageDiscoverDocs, build.ErrDiscovery, models.StageErrorWarning, true},
		{models.StageDiscoverDocs, build.ErrDiscovery, models.StageErrorFatal, false},
		{models.StageGenerateConfig, errors.New("cfg"), models.StageErrorFatal, false},
		{models.StageCopyContent, errors.New("io"), models.StageErrorFatal, false},
		// Typed transient git errors
		{models.StageCloneRepos, &gitpkg.RateLimitError{Op: "fetch", URL: "u", Err: errors.New("rate limit exceeded")}, models.StageErrorWarning, true},
		{models.StageCloneRepos, &gitpkg.NetworkTimeoutError{Op: "fetch", URL: "u", Err: errors.New("timeout")}, models.StageErrorWarning, true},
	}
	for i, c := range cases {
		se := &models.StageError{Stage: c.stage, Err: c.err, Kind: c.kind}
		if got := se.Transient(); got != c.want {
			t.Fatalf("case %d transient mismatch: got %v want %v (stage=%s kind=%s)", i, got, c.want, c.stage, c.kind)
		}
	}
}
