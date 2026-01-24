package lint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitUncommittedRenameDetector detects renames in the working tree and index
// (i.e., changes that may not be committed yet).
//
// It uses the git CLI because uncommitted renames are most naturally represented
// via the index/working-tree diffs.
//
// If repoRoot is not a git repository, it returns an empty slice and nil error.
//
// This is a building block for ADR-012.
type GitUncommittedRenameDetector struct{}

func (d *GitUncommittedRenameDetector) DetectRenames(ctx context.Context, repoRoot string) ([]RenameMapping, error) {
	repoRootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to make repo root absolute: %w", err)
	}

	isGit := isGitWorkTree(ctx, repoRootAbs)
	if !isGit {
		return nil, nil
	}

	staged, err := gitDiffRenames(ctx, repoRootAbs, true)
	if err != nil {
		return nil, err
	}

	// Best-effort: detect unstaged renames.
	// `git diff` does not consider untracked files, so a plain filesystem rename
	// often appears as "D old" + "?? new". We bridge that by matching deleted
	// index content to untracked file content.
	unstaged, err := detectUnstagedRenamesFromDeletedPlusUntracked(ctx, repoRootAbs)
	if err != nil {
		return nil, err
	}

	staged = append(staged, unstaged...)
	return NormalizeRenameMappings(staged, nil)
}

func isGitWorkTree(ctx context.Context, repoRoot string) bool {
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Not a git repo (or git unavailable). Treat as "no git" without error.
		return false
	}
	trimmed := bytes.TrimSpace(out)
	return bytes.Equal(trimmed, []byte("true"))
}

func gitDiffRenames(ctx context.Context, repoRoot string, cached bool) ([]RenameMapping, error) {
	args := []string{"-C", repoRoot, "diff", "--name-status", "-z", "-M"}
	if cached {
		args = append(args, "--cached")
	}

	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("git diff failed: %w: %s", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("git diff failed: %w", err)
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
				Source: RenameSourceGitUncommitted,
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

func detectUnstagedRenamesFromDeletedPlusUntracked(ctx context.Context, repoRoot string) ([]RenameMapping, error) {
	deletedRel, err := gitNameOnly(ctx, repoRoot, []string{"diff", "--name-only", "-z", "--diff-filter=D"})
	if err != nil {
		return nil, err
	}
	if len(deletedRel) == 0 {
		return nil, nil
	}

	untrackedRel, err := gitNameOnly(ctx, repoRoot, []string{"ls-files", "--others", "--exclude-standard", "-z"})
	if err != nil {
		return nil, err
	}
	if len(untrackedRel) == 0 {
		return nil, nil
	}

	// Hash untracked files by content.
	untrackedByHash := make(map[[32]byte][]string, len(untrackedRel))
	for _, rel := range untrackedRel {
		abs, ok := repoAbsPath(repoRoot, rel)
		if !ok {
			continue
		}

		// #nosec G304 -- path is validated to remain within repoRoot
		b, readErr := os.ReadFile(abs)
		if readErr != nil {
			continue
		}
		h := sha256.Sum256(b)
		untrackedByHash[h] = append(untrackedByHash[h], rel)
	}

	mappings := make([]RenameMapping, 0, len(deletedRel))
	for _, oldRel := range deletedRel {
		oldContent, err := gitShowIndexFile(ctx, repoRoot, oldRel)
		if err != nil {
			continue
		}
		oldHash := sha256.Sum256(oldContent)
		candidates := untrackedByHash[oldHash]
		if len(candidates) != 1 {
			// Ambiguous or no match.
			continue
		}
		newRel := candidates[0]
		oldAbs, okOld := repoAbsPath(repoRoot, oldRel)
		newAbs, okNew := repoAbsPath(repoRoot, newRel)
		if !okOld || !okNew {
			continue
		}
		mappings = append(mappings, RenameMapping{
			OldAbs: oldAbs,
			NewAbs: newAbs,
			Source: RenameSourceGitUncommitted,
		})
	}

	return mappings, nil
}

func gitNameOnly(ctx context.Context, repoRoot string, args []string) ([]string, error) {
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("git %v failed: %w: %s", args, err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("git %v failed: %w", args, err)
	}
	if len(out) == 0 {
		return nil, nil
	}

	parts := bytes.Split(out, []byte{0})
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		res = append(res, string(p))
	}
	return res, nil
}

func gitShowIndexFile(ctx context.Context, repoRoot, relPath string) ([]byte, error) {
	// `:<path>` reads the blob from the index.
	spec := ":" + relPath
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show", spec)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	b, readErr := io.ReadAll(stdout)
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		return nil, waitErr
	}
	return b, nil
}

func repoAbsPath(repoRoot, relPath string) (string, bool) {
	if relPath == "" {
		return "", false
	}
	if filepath.IsAbs(relPath) {
		return "", false
	}

	cleaned := filepath.Clean(filepath.FromSlash(relPath))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", false
	}

	abs := filepath.Join(repoRoot, cleaned)
	relToRoot, err := filepath.Rel(repoRoot, abs)
	if err != nil {
		return "", false
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", false
	}

	return abs, true
}
