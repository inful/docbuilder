package stages

import (
	"context"
	stdErrors "errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func StageCloneRepos(ctx context.Context, bs *models.BuildState) error {
	if len(bs.Git.Repositories) == 0 {
		return nil
	}
	if bs.Git.WorkspaceDir == "" {
		return models.NewFatalStageError(models.StageCloneRepos, stdErrors.New("workspace directory not set"))
	}
	fetcher := NewDefaultRepoFetcher(bs.Git.WorkspaceDir, &bs.Generator.Config().Build)
	// Ensure workspace directory structure (previously via git client)
	if err := os.MkdirAll(bs.Git.WorkspaceDir, 0o750); err != nil {
		return models.NewFatalStageError(models.StageCloneRepos, derrors.WrapError(err, derrors.CategoryFileSystem, "ensure workspace").Build())
	}
	strategy := config.CloneStrategyFresh
	if bs.Generator != nil {
		if s := bs.Generator.Config().Build.CloneStrategy; s != "" {
			strategy = s
		}
	}
	bs.Git.RepoPaths = make(map[string]string, len(bs.Git.Repositories))
	bs.Git.PreHeads = make(map[string]string, len(bs.Git.Repositories))
	bs.Git.PostHeads = make(map[string]string, len(bs.Git.Repositories))
	concurrency := 1
	if bs.Generator != nil && bs.Generator.Config().Build.CloneConcurrency > 0 {
		concurrency = bs.Generator.Config().Build.CloneConcurrency
	}
	if concurrency > len(bs.Git.Repositories) {
		concurrency = len(bs.Git.Repositories)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	type cloneTask struct{ repo config.Repository }
	tasks := make(chan cloneTask)
	var wg sync.WaitGroup
	var mu sync.Mutex
	worker := func() {
		defer wg.Done()
		for task := range tasks {
			select {
			case <-ctx.Done():
				return
			default:
			}
			start := time.Now()
			res := fetcher.Fetch(ctx, strategy, task.repo)
			_ = time.Since(start) // duration reserved for future per-repo metrics
			success := res.Err == nil
			mu.Lock()
			if success {
				recordCloneSuccess(bs, task.repo, res)
			} else {
				recordCloneFailure(bs, res)
			}
			mu.Unlock()
		}
	}
	wg.Add(concurrency)
	for range concurrency {
		go worker()
	}
	for i := range bs.Git.Repositories {
		r := &bs.Git.Repositories[i]
		select {
		case <-ctx.Done():
			close(tasks)
			wg.Wait()
			return models.NewCanceledStageError(models.StageCloneRepos, ctx.Err())
		default:
		}
		tasks <- cloneTask{repo: *r}
	}
	close(tasks)
	wg.Wait()
	select {
	case <-ctx.Done():
		return models.NewCanceledStageError(models.StageCloneRepos, ctx.Err())
	default:
	}
	bs.Git.AllReposUnchanged = bs.Git.AllReposUnchangedComputed()
	if bs.Git.AllReposUnchanged {
		slog.Info("No repository head changes detected", slog.Int("repos", len(bs.Git.PostHeads)))
	}
	if bs.Report.ClonedRepositories == 0 && bs.Report.FailedRepositories > 0 {
		return models.NewWarnStageError(models.StageCloneRepos, derrors.NewError(derrors.CategoryInternal, "all clones failed").WithCause(models.ErrClone).Build())
	}
	if bs.Report.FailedRepositories > 0 {
		return models.NewWarnStageError(models.StageCloneRepos, derrors.NewError(derrors.CategoryInternal, "clone failures exceeded budget").WithCause(models.ErrClone).WithContext("failed", bs.Report.FailedRepositories).WithContext("total", len(bs.Git.Repositories)).Build())
	}
	return nil
}

// classifyGitFailure inspects an error string for permanent git failure signatures.
func classifyGitFailure(err error) models.ReportIssueCode {
	if err == nil {
		return ""
	}

	// Use structured error classification (ADR-000)
	if ce, ok := derrors.AsClassified(err); ok {
		switch ce.Category() {
		case derrors.CategoryAuth:
			return models.IssueAuthFailure
		case derrors.CategoryNotFound:
			return models.IssueRepoNotFound
		case derrors.CategoryConfig:
			return models.IssueUnsupportedProto
		case derrors.CategoryNetwork:
			if ce.RetryStrategy() == derrors.RetryRateLimit {
				return models.IssueRateLimit
			}
			return models.IssueNetworkTimeout
		case derrors.CategoryValidation, derrors.CategoryAlreadyExists, derrors.CategoryGit,
			derrors.CategoryForge, derrors.CategoryBuild, derrors.CategoryHugo, derrors.CategoryFileSystem,
			derrors.CategoryDocs, derrors.CategoryEventStore, derrors.CategoryRuntime,
			derrors.CategoryDaemon, derrors.CategoryInternal:
			// Other categories use heuristic handling below
		}
		if diverged, ok := ce.Context().Get("diverged"); ok && diverged == true {
			return models.IssueRemoteDiverged
		}
	}

	// Fallback heuristic for legacy untyped errors
	l := strings.ToLower(err.Error())
	switch {
	case strings.Contains(l, "authentication failed") || strings.Contains(l, "authentication required") || strings.Contains(l, "invalid username or password") || strings.Contains(l, "authorization failed"):
		return models.IssueAuthFailure
	case strings.Contains(l, "repository not found") || (strings.Contains(l, "not found") && strings.Contains(l, "repository")):
		return models.IssueRepoNotFound
	case strings.Contains(l, "unsupported protocol"):
		return models.IssueUnsupportedProto
	case strings.Contains(l, "diverged") && strings.Contains(l, "hard reset disabled"):
		return models.IssueRemoteDiverged
	case strings.Contains(l, "rate limit") || strings.Contains(l, "too many requests"):
		return models.IssueRateLimit
	case strings.Contains(l, "timeout") || strings.Contains(l, "i/o timeout"):
		return models.IssueNetworkTimeout
	default:
		return ""
	}
}

// recordCloneSuccess updates build state after a successful repository clone.
func recordCloneSuccess(bs *models.BuildState, repo config.Repository, res RepoFetchResult) {
	bs.Report.ClonedRepositories++
	bs.Git.RepoPaths[repo.Name] = res.Path
	if res.PostHead != "" {
		bs.Git.SetPostHead(repo.Name, res.PostHead)
	}
	if res.PreHead != "" {
		bs.Git.SetPreHead(repo.Name, res.PreHead)
	}
}

// recordCloneFailure updates build state after a failed repository clone.
func recordCloneFailure(bs *models.BuildState, res RepoFetchResult) {
	bs.Report.FailedRepositories++
	if bs.Report != nil {
		code := classifyGitFailure(res.Err)
		if code != "" {
			bs.Report.AddIssue(code, models.StageName("clone_repos"), models.SeverityError, res.Err.Error(), false, res.Err)
		}
	}
}

// readRepoHead returns the current HEAD commit hash for a repository path.
//
// Deprecated: Use gitpkg.ReadRepoHead directly.
func readRepoHead(repoPath string) (string, error) {
	return gitpkg.ReadRepoHead(repoPath)
}
