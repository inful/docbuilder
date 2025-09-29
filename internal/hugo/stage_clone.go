package hugo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

func stageCloneRepos(ctx context.Context, bs *BuildState) error {
	if len(bs.Repositories) == 0 {
		return nil
	}
	if bs.WorkspaceDir == "" {
		return newFatalStageError(StageCloneRepos, fmt.Errorf("workspace directory not set"))
	}
	client := git.NewClient(bs.WorkspaceDir)
	if bs.Generator != nil {
		client = client.WithBuildConfig(&bs.Generator.config.Build)
	}
	strategy := config.CloneStrategyFresh
	if bs.Generator != nil {
		if s := bs.Generator.config.Build.CloneStrategy; s != "" {
			strategy = s
		}
	}
	if err := client.EnsureWorkspace(); err != nil {
		return newFatalStageError(StageCloneRepos, fmt.Errorf("ensure workspace: %w", err))
	}
	bs.RepoPaths = make(map[string]string, len(bs.Repositories))
	bs.preHeads = make(map[string]string, len(bs.Repositories))
	bs.postHeads = make(map[string]string, len(bs.Repositories))
	concurrency := 1
	if bs.Generator != nil && bs.Generator.config.Build.CloneConcurrency > 0 {
		concurrency = bs.Generator.config.Build.CloneConcurrency
	}
	if concurrency > len(bs.Repositories) {
		concurrency = len(bs.Repositories)
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
			var p string
			var err error
			attemptUpdate := false
			var preHead string
			switch strategy {
			case config.CloneStrategyUpdate:
				attemptUpdate = true
			case config.CloneStrategyAuto:
				repoPath := filepath.Join(bs.WorkspaceDir, task.repo.Name)
				if _, statErr := os.Stat(filepath.Join(repoPath, ".git")); statErr == nil {
					attemptUpdate = true
					if head, herr := readRepoHead(repoPath); herr == nil {
						preHead = head
					}
				}
			}
			if attemptUpdate {
				p, err = client.UpdateRepository(task.repo)
			} else {
				p, err = client.CloneRepository(task.repo)
			}
			dur := time.Since(start)
			success := err == nil
			mu.Lock()
			if success {
				bs.Report.ClonedRepositories++
				bs.RepoPaths[task.repo.Name] = p
				if head, herr := readRepoHead(p); herr == nil {
					bs.postHeads[task.repo.Name] = head
					if preHead != "" {
						bs.preHeads[task.repo.Name] = preHead
					}
				}
			} else {
				bs.Report.FailedRepositories++
				if bs.Report != nil {
					code := classifyGitFailure(err)
					if code != "" {
						bs.Report.AddIssue(code, StageCloneRepos, SeverityError, err.Error(), false, err)
					}
				}
			}
			mu.Unlock()
			if bs.Generator != nil && bs.Generator.recorder != nil {
				bs.Generator.recorder.ObserveCloneRepoDuration(task.repo.Name, dur, success)
				bs.Generator.recorder.IncCloneRepoResult(success)
			}
		}
	}
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}
	for _, r := range bs.Repositories {
		select {
		case <-ctx.Done():
			close(tasks)
			wg.Wait()
			return newCanceledStageError(StageCloneRepos, ctx.Err())
		default:
		}
		tasks <- cloneTask{repo: r}
	}
	close(tasks)
	wg.Wait()
	select {
	case <-ctx.Done():
		return newCanceledStageError(StageCloneRepos, ctx.Err())
	default:
	}
	unchanged := bs.Report.FailedRepositories == 0 && len(bs.postHeads) > 0
	if unchanged {
		for name, post := range bs.postHeads {
			if pre, ok := bs.preHeads[name]; !ok || pre == "" || pre != post {
				unchanged = false
				break
			}
		}
	}
	bs.AllReposUnchanged = unchanged
	if bs.AllReposUnchanged {
		slog.Info("No repository head changes detected", slog.Int("repos", len(bs.postHeads)))
	}
	if bs.Report.ClonedRepositories == 0 && bs.Report.FailedRepositories > 0 {
		return newWarnStageError(StageCloneRepos, fmt.Errorf("%w: all clones failed", build.ErrClone))
	}
	if bs.Report.FailedRepositories > 0 {
		return newWarnStageError(StageCloneRepos, fmt.Errorf("%w: %d failed out of %d", build.ErrClone, bs.Report.FailedRepositories, len(bs.Repositories)))
	}
	return nil
}

// classifyGitFailure inspects an error string for permanent git failure signatures.
func classifyGitFailure(err error) ReportIssueCode {
	if err == nil {
		return ""
	}
	l := strings.ToLower(err.Error())
	switch {
	case strings.Contains(l, "authentication failed") || strings.Contains(l, "authentication required") || strings.Contains(l, "invalid username or password") || strings.Contains(l, "authorization failed"):
		return IssueAuthFailure
	case strings.Contains(l, "repository not found") || strings.Contains(l, "not found") && strings.Contains(l, "repository"):
		return IssueRepoNotFound
	case strings.Contains(l, "unsupported protocol"):
		return IssueUnsupportedProto
	case strings.Contains(l, "diverged") && strings.Contains(l, "hard reset disabled"):
		return IssueRemoteDiverged
	default:
		return ""
	}
}

// readRepoHead returns the current HEAD commit hash for a repository path.
func readRepoHead(repoPath string) (string, error) {
	headPath := filepath.Join(repoPath, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "ref:"))
		refPath := filepath.Join(repoPath, ".git", filepath.FromSlash(ref))
		if b, berr := os.ReadFile(refPath); berr == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	return line, nil
}
