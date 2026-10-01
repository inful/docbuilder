package hugo

import (
	"errors"
	"testing"

	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func TestStageErrorTransient(t *testing.T) {
	cases := []struct {
		stage models.StageName
		err   error
		kind  models.StageErrorKind
		want  bool
	}{
		{models.StageCloneRepos, models.ErrClone, models.StageErrorWarning, true},
		{models.StageRunHugo, models.ErrHugo, models.StageErrorWarning, true},
		{models.StageDiscoverDocs, models.ErrDiscovery, models.StageErrorWarning, true},
		{models.StageDiscoverDocs, models.ErrDiscovery, models.StageErrorFatal, false},
		{models.StageGenerateConfig, errors.New("cfg"), models.StageErrorFatal, false},
		{models.StageCopyContent, errors.New("io"), models.StageErrorFatal, false},
		// Typed transient git errors
		{models.StageCloneRepos, gitpkg.ClassifyGitError(errors.New("rate limit exceeded"), "fetch", "u"), models.StageErrorWarning, true},
		{models.StageCloneRepos, gitpkg.ClassifyGitError(errors.New("network timeout"), "fetch", "u"), models.StageErrorWarning, true},
	}
	for i, c := range cases {
		se := &models.StageError{Stage: c.stage, Err: c.err, Kind: c.kind}
		if got := se.Transient(); got != c.want {
			t.Fatalf("case %d transient mismatch: got %v want %v (stage=%s kind=%s)", i, got, c.want, c.stage, c.kind)
		}
	}
}
