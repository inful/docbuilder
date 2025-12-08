package git

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RepoTree represents a snapshot of a repository at a specific commit.
type RepoTree struct {
	RepoPath string   // Path to the repository
	Commit   string   // Commit SHA
	Paths    []string // Configured documentation paths to include
	Hash     string   // Computed content hash
}

// ComputeRepoHash computes a deterministic hash for a repository tree.
// The hash is based on:
// - The commit SHA
// - The tree structure and content of configured paths
//
// This enables content-addressable caching: same commit + same paths = same hash.
func ComputeRepoHash(repoPath, commit string, paths []string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}

	// Resolve commit
	hash, err := repo.ResolveRevision(plumbing.Revision(commit))
	if err != nil {
		return "", fmt.Errorf("resolve commit %s: %w", commit, err)
	}

	commitObj, err := repo.CommitObject(*hash)
	if err != nil {
		return "", fmt.Errorf("get commit object: %w", err)
	}

	tree, err := commitObj.Tree()
	if err != nil {
		return "", fmt.Errorf("get tree: %w", err)
	}

	// Build list of files to hash
	var fileHashes []string

	// If no paths specified, use entire tree
	if len(paths) == 0 {
		paths = []string{"."}
	}

	for _, path := range paths {
		// Handle root path
		if path == "." || path == "" {
			// Hash entire tree
			if err := hashTree(tree, "", &fileHashes); err != nil {
				return "", fmt.Errorf("hash tree: %w", err)
			}
			continue
		}

		// Hash specific subtree
		entry, err := tree.FindEntry(path)
		if err != nil {
			// Path doesn't exist in this commit, skip
			continue
		}

		if entry.Mode.IsFile() {
			// Single file
			fileHashes = append(fileHashes, fmt.Sprintf("%s:%s", path, entry.Hash.String()))
		} else {
			// Subtree
			subtree, err := tree.Tree(path)
			if err != nil {
				continue
			}
			if err := hashTree(subtree, path, &fileHashes); err != nil {
				return "", fmt.Errorf("hash subtree %s: %w", path, err)
			}
		}
	}

	// Sort for deterministic ordering
	sort.Strings(fileHashes)

	// Compute final hash
	h := sha256.New()
	h.Write([]byte(commitObj.Hash.String())) // Include commit SHA
	for _, fh := range fileHashes {
		h.Write([]byte(fh))
		h.Write([]byte("\n"))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashTree recursively hashes all files in a tree.
func hashTree(tree *object.Tree, prefix string, fileHashes *[]string) error {
	return tree.Files().ForEach(func(file *object.File) error {
		path := file.Name
		if prefix != "" {
			path = filepath.Join(prefix, file.Name)
		}
		*fileHashes = append(*fileHashes, fmt.Sprintf("%s:%s", path, file.Hash.String()))
		return nil
	})
}

// ComputeRepoHashFromWorkdir computes a hash from the working directory.
// This is useful when the repository hasn't been cloned via go-git.
// It walks the filesystem and hashes file content.
func ComputeRepoHashFromWorkdir(repoPath string, paths []string) (string, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var fileHashes []string

	for _, path := range paths {
		fullPath := filepath.Join(repoPath, path)

		// Check if path exists
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Path doesn't exist, skip
			}
			return "", fmt.Errorf("stat %s: %w", fullPath, err)
		}

		if info.IsDir() {
			// Walk directory
			err := filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip directories and hidden files
				if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
					return nil
				}

				// Compute file hash
				// #nosec G304 - p is from filepath.Walk, within controlled directory
				content, err := os.ReadFile(p)
				if err != nil {
					return fmt.Errorf("read %s: %w", p, err)
				}

				h := sha256.Sum256(content)
				relPath, _ := filepath.Rel(repoPath, p)
				fileHashes = append(fileHashes, fmt.Sprintf("%s:%s", relPath, hex.EncodeToString(h[:])))
				return nil
			})
			if err != nil {
				return "", fmt.Errorf("walk %s: %w", fullPath, err)
			}
		} else {
			// Single file
			// #nosec G304 - fullPath is validated and within controlled directory
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return "", fmt.Errorf("read %s: %w", fullPath, err)
			}

			h := sha256.Sum256(content)
			relPath, _ := filepath.Rel(repoPath, fullPath)
			fileHashes = append(fileHashes, fmt.Sprintf("%s:%s", relPath, hex.EncodeToString(h[:])))
		}
	}

	// Sort for deterministic ordering
	sort.Strings(fileHashes)

	// Compute final hash
	h := sha256.New()
	for _, fh := range fileHashes {
		h.Write([]byte(fh))
		h.Write([]byte("\n"))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetRepoTree computes and returns a RepoTree for the given repository.
func GetRepoTree(repoPath, commit string, paths []string) (*RepoTree, error) {
	hash, err := ComputeRepoHash(repoPath, commit, paths)
	if err != nil {
		return nil, err
	}

	return &RepoTree{
		RepoPath: repoPath,
		Commit:   commit,
		Paths:    paths,
		Hash:     hash,
	}, nil
}
