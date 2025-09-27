package hugo

import (
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build"
)

func TestStageErrorTransient(t *testing.T) {
	cases := []struct {
		stage StageName
		err   error
		kind  StageErrorKind
		want  bool
	}{
		{StageCloneRepos, build.ErrClone, StageErrorWarning, true},
		{StageRunHugo, build.ErrHugo, StageErrorWarning, true},
		{StageDiscoverDocs, build.ErrDiscovery, StageErrorWarning, true},
		{StageDiscoverDocs, build.ErrDiscovery, StageErrorFatal, false},
		{StageGenerateConfig, errors.New("cfg"), StageErrorFatal, false},
		{StageCopyContent, errors.New("io"), StageErrorFatal, false},
	}
	for i, c := range cases {
		se := &StageError{Stage: c.stage, Err: c.err, Kind: c.kind}
		if got := se.Transient(); got != c.want {
			t.Fatalf("case %d transient mismatch: got %v want %v (stage=%s kind=%s)", i, got, c.want, c.stage, c.kind)
		}
	}
}
