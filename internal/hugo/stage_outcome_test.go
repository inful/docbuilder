package hugo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	cfgpkg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// minimal build state helper.
func newTestBuildState() *models.BuildState {
	cfg := &cfgpkg.Config{Hugo: cfgpkg.HugoConfig{Title: "T"}}
	g := NewGenerator(cfg, "")
	rep := models.NewBuildReport(context.Background(), 0, 0)
	return models.NewBuildState(g, nil, rep)
}

func TestClassifyStageResult_Success(t *testing.T) {
	bs := newTestBuildState()
	out := stages.ClassifyStageResult(models.StageCopyContent, nil, bs)
	if out.Result != models.StageResultSuccess || out.Error != nil || out.Abort {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestClassifyStageResult_WrappedClonePartial(t *testing.T) {
	bs := newTestBuildState()
	bs.Report.ClonedRepositories = 1
	bs.Report.FailedRepositories = 1
	wrapped := fmt.Errorf("wrap: %w", build.ErrClone)
	se := models.NewWarnStageError(models.StageCloneRepos, wrapped)
	out := stages.ClassifyStageResult(models.StageCloneRepos, se, bs)
	if out.IssueCode != models.IssuePartialClone {
		t.Fatalf("expected partial clone, got %s", out.IssueCode)
	}
	if out.Result != models.StageResultWarning || out.Abort {
		t.Fatalf("expected warning non-abort: %+v", out)
	}
}

func TestClassifyStageResult_UnknownFatal(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("boom")
	out := stages.ClassifyStageResult(models.StageRunHugo, err, bs)
	if out.IssueCode != models.IssueGenericStageError {
		t.Fatalf("expected generic code, got %s", out.IssueCode)
	}
	if out.Result != models.StageResultFatal || !out.Abort {
		t.Fatalf("expected fatal abort %+v", out)
	}
}

// TestClassifyStageResult_AllClonesFailed tests when all clones fail.
func TestClassifyStageResult_AllClonesFailed(t *testing.T) {
	bs := newTestBuildState()
	bs.Report.ClonedRepositories = 0
	bs.Report.FailedRepositories = 3
	wrapped := fmt.Errorf("wrap: %w", build.ErrClone)
	se := models.NewWarnStageError(models.StageCloneRepos, wrapped)
	out := stages.ClassifyStageResult(models.StageCloneRepos, se, bs)
	if out.IssueCode != models.IssueAllClonesFailed {
		t.Fatalf("expected all clones failed, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_CloneFailureNonStandard tests clone failure without build.ErrClone.
func TestClassifyStageResult_CloneFailureNonStandard(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("some other clone error")
	se := models.NewWarnStageError(models.StageCloneRepos, err)
	out := stages.ClassifyStageResult(models.StageCloneRepos, se, bs)
	if out.IssueCode != models.IssueCloneFailure {
		t.Fatalf("expected clone failure, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_NoRepositories tests discovery when no repos found.
func TestClassifyStageResult_NoRepositories(t *testing.T) {
	bs := newTestBuildState()
	bs.Git.RepoPaths = make(map[string]string) // empty
	wrapped := fmt.Errorf("wrap: %w", build.ErrDiscovery)
	se := models.NewWarnStageError(models.StageDiscoverDocs, wrapped)
	out := stages.ClassifyStageResult(models.StageDiscoverDocs, se, bs)
	if out.IssueCode != models.IssueNoRepositories {
		t.Fatalf("expected no repositories, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_DiscoveryFailure tests discovery failure with repos.
func TestClassifyStageResult_DiscoveryFailure(t *testing.T) {
	bs := newTestBuildState()
	bs.Git.RepoPaths = map[string]string{"repo1": "/path/to/repo"}
	wrapped := fmt.Errorf("wrap: %w", build.ErrDiscovery)
	se := models.NewWarnStageError(models.StageDiscoverDocs, wrapped)
	out := stages.ClassifyStageResult(models.StageDiscoverDocs, se, bs)
	if out.IssueCode != models.IssueDiscoveryFailure {
		t.Fatalf("expected discovery failure, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_DiscoveryFailureNonStandard tests discovery error without build.ErrDiscovery.
func TestClassifyStageResult_DiscoveryFailureNonStandard(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("some other discovery error")
	se := models.NewWarnStageError(models.StageDiscoverDocs, err)
	out := stages.ClassifyStageResult(models.StageDiscoverDocs, se, bs)
	if out.IssueCode != models.IssueDiscoveryFailure {
		t.Fatalf("expected discovery failure, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_HugoExecution tests hugo execution failure.
func TestClassifyStageResult_HugoExecution(t *testing.T) {
	bs := newTestBuildState()
	wrapped := fmt.Errorf("wrap: %w", build.ErrHugo)
	se := models.NewWarnStageError(models.StageRunHugo, wrapped)
	out := stages.ClassifyStageResult(models.StageRunHugo, se, bs)
	if out.IssueCode != models.IssueHugoExecution {
		t.Fatalf("expected hugo execution, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_HugoExecutionNonStandard tests hugo error without build.ErrHugo.
func TestClassifyStageResult_HugoExecutionNonStandard(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("some other hugo error")
	se := models.NewWarnStageError(models.StageRunHugo, err)
	out := stages.ClassifyStageResult(models.StageRunHugo, se, bs)
	if out.IssueCode != models.IssueHugoExecution {
		t.Fatalf("expected hugo execution, got %s", out.IssueCode)
	}
}

// TestClassifyStageResult_Canceled tests canceled stage.
func TestClassifyStageResult_Canceled(t *testing.T) {
	bs := newTestBuildState()
	err := errors.New("canceled")
	se := models.NewCanceledStageError(models.StageRunHugo, err)
	out := stages.ClassifyStageResult(models.StageRunHugo, se, bs)
	if out.IssueCode != models.IssueCanceled {
		t.Fatalf("expected canceled, got %s", out.IssueCode)
	}
	if !out.Abort {
		t.Fatalf("expected abort for canceled stage")
	}
}
