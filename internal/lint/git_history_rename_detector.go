package lint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

const defaultHistoryFallbackCommits = 50

// GitHistoryRenameDetector detects renames that have already been committed
// in Git history, typically for commits that exist locally but are not yet
// present on the upstream tracking branch.
//
// This is a building block for ADR-012.
//
// Behavior:
//   - If repoRoot is not a git repository, it returns an empty slice and nil error.
//   - If an upstream tracking branch exists, it uses the range upstream..HEAD.
//   - If upstream is absent, it uses a bounded fallback range based on the last
//     N commits (defaultHistoryFallbackCommits).
//
// It uses the git CLI to leverage Git's rename detection.
type GitHistoryRenameDetector struct {
	// MaxCommits bounds the fallback range when upstream is absent.
	// If zero, defaultHistoryFallbackCommits is used.
	MaxCommits int
}

func (d *GitHistoryRenameDetector) DetectRenames(ctx context.Context, repoRoot string) ([]RenameMapping, error) {
	repoRootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to make repo root absolute: %w", err)
	}

	isGit := isGitWorkTree(ctx, repoRootAbs)
	if !isGit {
		return nil, nil
	}

	upstream, hasUpstream := gitUpstreamRef(ctx, repoRootAbs)
	if hasUpstream {
		mappings, diffErr := gitDiffRenamesRange(ctx, repoRootAbs, upstream+"..HEAD")
		if diffErr != nil {
			return nil, diffErr
		}
		for i := range mappings {
			mappings[i].Source = RenameSourceGitHistory
		}
		return NormalizeRenameMappings(mappings, nil)
	}

	maxCommits := d.MaxCommits
	if maxCommits <= 0 {
		maxCommits = defaultHistoryFallbackCommits
	}

	base, ok := gitFallbackBaseCommit(ctx, repoRootAbs, maxCommits)
	if !ok {
		return nil, nil
	}

	mappings, err := gitDiffRenamesRange(ctx, repoRootAbs, base+"..HEAD")
	if err != nil {
		return nil, err
	}
	for i := range mappings {
		mappings[i].Source = RenameSourceGitHistory
	}
	return NormalizeRenameMappings(mappings, nil)
}

func gitUpstreamRef(ctx context.Context, repoRoot string) (string, bool) {
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return "", false
	}
	return string(trimmed), true
}

func gitFallbackBaseCommit(ctx context.Context, repoRoot string, maxCommits int) (base string, ok bool) {
	if maxCommits <= 0 {
		return "", false
	}

	// Determine whether HEAD~(maxCommits) exists; if it doesn't (small history),
	// HEAD~1 may still exist.
	for n := maxCommits; n >= 1; n-- {
		candidate := fmt.Sprintf("HEAD~%d", n)
		if gitRevParseOK(ctx, repoRoot, candidate) {
			return candidate, true
		}
	}

	// No ancestors (repo with 0 or 1 commit).
	return "", false
}

func gitRevParseOK(ctx context.Context, repoRoot string, rev string) bool {
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "-q", rev)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func gitDiffRenamesRange(ctx context.Context, repoRoot string, rangeSpec string) ([]RenameMapping, error) {
	args := []string{"-C", repoRoot, "diff", "--name-status", "-z", "-M", rangeSpec}

	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("git diff %s failed: %w: %s", rangeSpec, err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("git diff %s failed: %w", rangeSpec, err)
	}

	if len(out) == 0 {
		return nil, nil
	}

	tokens := bytes.Split(out, []byte{0})
	mappings := make([]RenameMapping, 0)
	for i := 0; i < len(tokens); {
		if len(tokens[i]) == 0 {
			i++
			continue
		}
		status := string(tokens[i])
		i++

		if len(status) > 0 && status[0] == 'R' {
			if i+1 >= len(tokens) {
				break
			}
			oldRel := string(tokens[i])
			newRel := string(tokens[i+1])
			i += 2

			oldAbs, okOld := repoAbsPath(repoRoot, oldRel)
			newAbs, okNew := repoAbsPath(repoRoot, newRel)
			if !okOld || !okNew {
				continue
			}

			mappings = append(mappings, RenameMapping{
				OldAbs: oldAbs,
				NewAbs: newAbs,
				Source: RenameSourceGitHistory,
			})
			continue
		}

		// Non-rename entries have a single path token.
		if i < len(tokens) {
			i++
		}
	}

	return mappings, nil
}
