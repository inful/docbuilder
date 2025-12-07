package git

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadRepoHead returns the current HEAD commit hash for a git repository.
// It reads .git/HEAD and resolves symbolic references if needed.
func ReadRepoHead(repoPath string) (string, error) {
	headPath := filepath.Join(repoPath, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}

	line := strings.TrimSpace(string(data))

	// If HEAD is a symbolic ref (e.g., "ref: refs/heads/main"), resolve it
	if strings.HasPrefix(line, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "ref:"))
		refPath := filepath.Join(repoPath, ".git", filepath.FromSlash(ref))
		if refData, refErr := os.ReadFile(refPath); refErr == nil {
			return strings.TrimSpace(string(refData)), nil
		}
	}

	// Otherwise, HEAD contains the commit hash directly
	return line, nil
}
