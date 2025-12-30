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

// TestClassifyStageResult_AllClonesFailed tests when all clones fail
func TestClassifyStageResult_AllClonesFailed(t *testing.T) {
	bs := newTestBuildState()
	bs.Report.ClonedRepositories = 0
	bs.Report.FailedRepositories = 3
	wrapped := fmt.Errorf("wrap: %w", build.ErrClone)
	se := newWarnStageError(StageCloneRepos, wrapped)
	out := classifyStageResult(StageCloneRepos, se, bs)
	if out.IssueCode != IssueAllClonesFailed {
		t.Fatalf("expected all clones failed, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_CloneFailureNonStandard tests clone failure without build.ErrClone
func TestClassifyStageResult_CloneFailureNonStandard(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("some other clone error")
	se := newWarnStageError(StageCloneRepos, err)
	out := classifyStageResult(StageCloneRepos, se, bs)
	if out.IssueCode != IssueCloneFailure {
		t.Fatalf("expected clone failure, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_NoRepositories tests discovery when no repos found
func TestClassifyStageResult_NoRepositories(t *testing.T) {
	bs := newTestBuildState()
	bs.Git.RepoPaths = make(map[string]string) // empty
	wrapped := fmt.Errorf("wrap: %w", build.ErrDiscovery)
	se := newWarnStageError(StageDiscoverDocs, wrapped)
	out := classifyStageResult(StageDiscoverDocs, se, bs)
	if out.IssueCode != IssueNoRepositories {
		t.Fatalf("expected no repositories, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_DiscoveryFailure tests discovery failure with repos
func TestClassifyStageResult_DiscoveryFailure(t *testing.T) {
	bs := newTestBuildState()
	bs.Git.RepoPaths = map[string]string{"repo1": "/path/to/repo"}
	wrapped := fmt.Errorf("wrap: %w", build.ErrDiscovery)
	se := newWarnStageError(StageDiscoverDocs, wrapped)
	out := classifyStageResult(StageDiscoverDocs, se, bs)
	if out.IssueCode != IssueDiscoveryFailure {
		t.Fatalf("expected discovery failure, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_DiscoveryFailureNonStandard tests discovery error without build.ErrDiscovery
func TestClassifyStageResult_DiscoveryFailureNonStandard(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("some other discovery error")
	se := newWarnStageError(StageDiscoverDocs, err)
	out := classifyStageResult(StageDiscoverDocs, se, bs)
	if out.IssueCode != IssueDiscoveryFailure {
		t.Fatalf("expected discovery failure, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_HugoExecution tests hugo execution failure
func TestClassifyStageResult_HugoExecution(t *testing.T) {
	bs := newTestBuildState()
	wrapped := fmt.Errorf("wrap: %w", build.ErrHugo)
	se := newWarnStageError(StageRunHugo, wrapped)
	out := classifyStageResult(StageRunHugo, se, bs)
	if out.IssueCode != IssueHugoExecution {
		t.Fatalf("expected hugo execution, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_HugoExecutionNonStandard tests hugo error without build.ErrHugo
func TestClassifyStageResult_HugoExecutionNonStandard(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("some other hugo error")
	se := newWarnStageError(StageRunHugo, err)
	out := classifyStageResult(StageRunHugo, se, bs)
	if out.IssueCode != IssueHugoExecution {
		t.Fatalf("expected hugo execution, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_Canceled tests canceled stage
func TestClassifyStageResult_Canceled(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("canceled")
	se := newCanceledStageError(StageRunHugo, err)
	out := classifyStageResult(StageRunHugo, se, bs)
	if out.IssueCode != IssueCanceled {
		t.Fatalf("expected canceled, got %s", out.IssueCode)
	}
	if !out.Abort {
		t.Fatalf("expected abort for canceled stage")
	}
}
