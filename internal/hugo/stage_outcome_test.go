package hugo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	cfgpkg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// minimal build state helper.
func newTestBuildState() *BuildState {
	cfg := &cfgpkg.Config{Hugo: cfgpkg.HugoConfig{Title: "T"}}
	g := NewGenerator(cfg, "")
	rep := newBuildReport(context.Background(), 0, 0)
	return newBuildState(g, nil, rep)
}

func TestClassifyStageResult_Success(t *testing.T) {
	bs := newTestBuildState()
	out := classifyStageResult(StageCopyContent, nil, bs)
	if out.Result != StageResultSuccess || out.Error != nil || out.Abort {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestClassifyStageResult_WrappedClonePartial(t *testing.T) {
	bs := newTestBuildState()
	bs.Report.ClonedRepositories = 1
	bs.Report.FailedRepositories = 1
	wrapped := fmt.Errorf("wrap: %w", build.ErrClone)
	se := newWarnStageError(StageCloneRepos, wrapped)
	out := classifyStageResult(StageCloneRepos, se, bs)
	if out.IssueCode != IssuePartialClone {
		t.Fatalf("expected partial clone, got %s", out.IssueCode)
	}
	if out.Result != StageResultWarning || out.Abort {
		t.Fatalf("expected warning non-abort: %+v", out)
	}
}

func TestClassifyStageResult_UnknownFatal(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("boom")
	out := classifyStageResult(StageRunHugo, err, bs)
	if out.IssueCode != IssueGenericStageError {
		t.Fatalf("expected generic code, got %s", out.IssueCode)
	}
	if out.Result != StageResultFatal || !out.Abort {
		t.Fatalf("expected fatal abort %+v", out)
	}
}
