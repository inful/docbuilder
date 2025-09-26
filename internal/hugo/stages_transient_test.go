package hugo

import (
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build"
)

func TestStageErrorTransient(t *testing.T) {
	cases := []struct {
		stage string
		err   error
		kind  StageErrorKind
		want  bool
	}{
		{"clone_repos", build.ErrClone, StageErrorWarning, true},
		{"run_hugo", build.ErrHugo, StageErrorWarning, true},
		{"discover_docs", build.ErrDiscovery, StageErrorWarning, true},
		{"discover_docs", build.ErrDiscovery, StageErrorFatal, false},
		{"generate_config", errors.New("cfg"), StageErrorFatal, false},
		{"copy_content", errors.New("io"), StageErrorFatal, false},
	}
	for i, c := range cases {
		se := &StageError{Stage: c.stage, Err: c.err, Kind: c.kind}
		if got := se.Transient(); got != c.want {
			t.Fatalf("case %d transient mismatch: got %v want %v (stage=%s kind=%s)", i, got, c.want, c.stage, c.kind)
		}
	}
}
