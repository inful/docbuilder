package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
)

// CloneReposCommand implements the repository cloning stage.
type CloneReposCommand struct {
	BaseCommand
}

// NewCloneReposCommand creates a new clone repos command.
func NewCloneReposCommand() *CloneReposCommand {
	return &CloneReposCommand{
		BaseCommand: NewBaseCommand(CommandMetadata{
			Name:        models.StageCloneRepos,
			Description: "Clone and update configured repositories",
			Dependencies: []models.StageName{
				models.StagePrepareOutput, // Depends on workspace preparation
			},
			SkipIf: func(bs *models.BuildState) bool {
				return len(bs.Git.Repositories) == 0
			},
		}),
	}
}

// Execute runs the clone repos stage.
func (c *CloneReposCommand) Execute(ctx context.Context, bs *models.BuildState) stages.StageExecution {
	c.LogStageStart()

	if bs.Git.WorkspaceDir == "" {
		err := errors.New("workspace directory not set")
		c.LogStageFailure(err)
		return stages.ExecutionFailure(err)
	}

	fetcher := stages.NewDefaultRepoFetcher(bs.Git.WorkspaceDir, &bs.Generator.Config().Build)

	// Ensure workspace directory structure
	if err := os.MkdirAll(bs.Git.WorkspaceDir, 0o750); err != nil {
		err = fmt.Errorf("ensure workspace: %w", err)
		c.LogStageFailure(err)
		return stages.ExecutionFailure(err)
	}

	strategy := config.CloneStrategyFresh
	if bs.Generator != nil {
		if s := bs.Generator.Config().Build.CloneStrategy; s != "" {
			strategy = s
		}
	}

	bs.Git.RepoPaths = make(map[string]string, len(bs.Git.Repositories))
	// Note: preHeads and postHeads are private fields that should be initialized by models.BuildState constructor
	// In the command pattern, we skip this initialization and rely on proper models.BuildState setup

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

	// Record concurrency if metrics are available (handled by metrics infrastructure)

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
				c.recordSuccessfulClone(bs, task.repo, res)
			} else {
				c.recordFailedClone(bs, res)
			}
			mu.Unlock()

			// Metrics recording handled by infrastructure
			_ = dur
			_ = success
		}
	}

	wg.Add(concurrency)
	for range concurrency {
		go worker()
	}

	for i := range bs.Git.Repositories {
		select {
		case <-ctx.Done():
			close(tasks)
			wg.Wait()
			err := ctx.Err()
			c.LogStageFailure(err)
			return stages.ExecutionFailure(err)
		default:
		}
		tasks <- cloneTask{repo: bs.Git.Repositories[i]}
	}

	close(tasks)
	wg.Wait()

	select {
	case <-ctx.Done():
		err := ctx.Err()
		c.LogStageFailure(err)
		return stages.ExecutionFailure(err)
	default:
	}

	bs.Git.AllReposUnchanged = bs.Git.AllReposUnchangedComputed()
	if bs.Git.AllReposUnchanged {
		slog.Info("No repository head changes detected", slog.Int("repos", len(bs.Git.Repositories)))
	}

	if bs.Report.ClonedRepositories == 0 && bs.Report.FailedRepositories > 0 {
		err := fmt.Errorf("%w: all clones failed", models.ErrClone)
		c.LogStageFailure(err)
		return stages.ExecutionFailure(err)
	}

	if bs.Report.FailedRepositories > 0 {
		// This is a warning, not a fatal error
		slog.Warn("Some repositories failed to clone",
			slog.Int("failed", bs.Report.FailedRepositories),
			slog.Int("total", len(bs.Git.Repositories)))
	}

	c.LogStageSuccess()
	return stages.ExecutionSuccess()
}

// recordSuccessfulClone updates build state after a successful repository clone.
func (c *CloneReposCommand) recordSuccessfulClone(bs *models.BuildState, repo config.Repository, res stages.RepoFetchResult) {
	bs.Report.ClonedRepositories++
	bs.Git.RepoPaths[repo.Name] = res.Path
	if res.PostHead != "" {
		bs.Git.SetPostHead(repo.Name, res.PostHead)
	}
	if res.PreHead != "" {
		bs.Git.SetPreHead(repo.Name, res.PreHead)
	}
	if !res.CommitDate.IsZero() {
		bs.Git.SetCommitDate(repo.Name, res.CommitDate)
	}
}

// recordFailedClone updates build state after a failed repository clone.
func (c *CloneReposCommand) recordFailedClone(bs *models.BuildState, res stages.RepoFetchResult) {
	bs.Report.FailedRepositories++
	if bs.Report != nil {
		code := c.classifyGitFailure(res.Err)
		if code != "" {
			bs.Report.AddIssue(code, models.StageCloneRepos, models.SeverityError, res.Err.Error(), false, res.Err)
		}
	}
}

// classifyGitFailure inspects an error string for permanent git failure signatures.
func (c *CloneReposCommand) classifyGitFailure(err error) models.ReportIssueCode {
	if err == nil {
		return ""
	}

	// Prefer typed errors first
	switch {
	case errors.As(err, new(*gitpkg.AuthError)):
		return models.IssueAuthFailure
	case errors.As(err, new(*gitpkg.NotFoundError)):
		return models.IssueRepoNotFound
	case errors.As(err, new(*gitpkg.UnsupportedProtocolError)):
		return models.IssueUnsupportedProto
	case errors.As(err, new(*gitpkg.RemoteDivergedError)):
		return models.IssueRemoteDiverged
	case errors.As(err, new(*gitpkg.RateLimitError)):
		return models.IssueRateLimit
	case errors.As(err, new(*gitpkg.NetworkTimeoutError)):
		return models.IssueNetworkTimeout
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
