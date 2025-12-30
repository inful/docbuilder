package hugo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
)

func stageCloneRepos(ctx context.Context, bs *BuildState) error {
	if len(bs.Git.Repositories) == 0 {
		return nil
	}
	if bs.Git.WorkspaceDir == "" {
		return newFatalStageError(StageCloneRepos, errors.New("workspace directory not set"))
	}
	fetcher := NewDefaultRepoFetcher(bs.Git.WorkspaceDir, &bs.Generator.config.Build)
	// Ensure workspace directory structure (previously via git client)
	if err := os.MkdirAll(bs.Git.WorkspaceDir, 0o750); err != nil {
		return newFatalStageError(StageCloneRepos, fmt.Errorf("ensure workspace: %w", err))
	}
	strategy := config.CloneStrategyFresh
	if bs.Generator != nil {
		if s := bs.Generator.config.Build.CloneStrategy; s != "" {
			strategy = s
		}
	}
	bs.Git.RepoPaths = make(map[string]string, len(bs.Git.Repositories))
	bs.Git.preHeads = make(map[string]string, len(bs.Git.Repositories))
	bs.Git.postHeads = make(map[string]string, len(bs.Git.Repositories))
	concurrency := 1
	if bs.Generator != nil && bs.Generator.config.Build.CloneConcurrency > 0 {
		concurrency = bs.Generator.config.Build.CloneConcurrency
	}
	if concurrency > len(bs.Git.Repositories) {
		concurrency = len(bs.Git.Repositories)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if bs.Generator != nil && bs.Generator.recorder != nil {
		bs.Generator.recorder.SetCloneConcurrency(concurrency)
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
			dur := time.Since(start)
			success := res.Err == nil
			mu.Lock()
			if success {
				recordCloneSuccess(bs, task.repo, res)
			} else {
				recordCloneFailure(bs, res)
			}
			mu.Unlock()
			if bs.Generator != nil && bs.Generator.recorder != nil {
				bs.Generator.recorder.ObserveCloneRepoDuration(task.repo.Name, dur, success)
				bs.Generator.recorder.IncCloneRepoResult(success)
			}
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
			return newCanceledStageError(StageCloneRepos, ctx.Err())
		default:
		}
		tasks <- cloneTask{repo: *r}
	}
	close(tasks)
	wg.Wait()
	select {
	case <-ctx.Done():
		return newCanceledStageError(StageCloneRepos, ctx.Err())
	default:
	}
	bs.Git.AllReposUnchanged = bs.Git.AllReposUnchangedComputed()
	if bs.Git.AllReposUnchanged {
		slog.Info("No repository head changes detected", slog.Int("repos", len(bs.Git.postHeads)))
	}
	if bs.Report.ClonedRepositories == 0 && bs.Report.FailedRepositories > 0 {
		return newWarnStageError(StageCloneRepos, fmt.Errorf("%w: all clones failed", build.ErrClone))
	}
	if bs.Report.FailedRepositories > 0 {
		return newWarnStageError(StageCloneRepos, fmt.Errorf("%w: %d failed out of %d", build.ErrClone, bs.Report.FailedRepositories, len(bs.Git.Repositories)))
	}
	return nil
}

// classifyGitFailure inspects an error string for permanent git failure signatures.
func classifyGitFailure(err error) ReportIssueCode {
	if err == nil {
		return ""
	}
	// Prefer typed errors (Phase 4) first
	switch {
	case errors.As(err, new(*gitpkg.AuthError)):
		return IssueAuthFailure
	case errors.As(err, new(*gitpkg.NotFoundError)):
		return IssueRepoNotFound
	case errors.As(err, new(*gitpkg.UnsupportedProtocolError)):
		return IssueUnsupportedProto
	case errors.As(err, new(*gitpkg.RemoteDivergedError)):
		return IssueRemoteDiverged
	case errors.As(err, new(*gitpkg.RateLimitError)):
		return IssueRateLimit
	case errors.As(err, new(*gitpkg.NetworkTimeoutError)):
		return IssueNetworkTimeout
	}
	// Fallback heuristic for legacy untyped errors
	l := strings.ToLower(err.Error())
	switch {
	case strings.Contains(l, "authentication failed") || strings.Contains(l, "authentication required") || strings.Contains(l, "invalid username or password") || strings.Contains(l, "authorization failed"):
		return IssueAuthFailure
	case strings.Contains(l, "repository not found") || (strings.Contains(l, "not found") && strings.Contains(l, "repository")):
		return IssueRepoNotFound
	case strings.Contains(l, "unsupported protocol"):
		return IssueUnsupportedProto
	case strings.Contains(l, "diverged") && strings.Contains(l, "hard reset disabled"):
		return IssueRemoteDiverged
	case strings.Contains(l, "rate limit") || strings.Contains(l, "too many requests"):
		return IssueRateLimit
	case strings.Contains(l, "timeout") || strings.Contains(l, "i/o timeout"):
		return IssueNetworkTimeout
	default:
		return ""
	}
}

// recordCloneSuccess updates build state after a successful repository clone.
func recordCloneSuccess(bs *BuildState, repo config.Repository, res RepoFetchResult) {
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
func recordCloneFailure(bs *BuildState, res RepoFetchResult) {
	bs.Report.FailedRepositories++
	if bs.Report != nil {
		code := classifyGitFailure(res.Err)
		if code != "" {
			bs.Report.AddIssue(code, StageCloneRepos, SeverityError, res.Err.Error(), false, res.Err)
		}
	}
}

// readRepoHead returns the current HEAD commit hash for a repository path.
//
// Deprecated: Use gitpkg.ReadRepoHead directly.
func readRepoHead(repoPath string) (string, error) {
	return gitpkg.ReadRepoHead(repoPath)
}
