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

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
		return "", foundationerrors.WrapError(err, foundationerrors.CategoryGit, "failed to open repository").WithSeverity(foundationerrors.SeverityError).WithContext("repo_path", repoPath).Build()
	}

	// Resolve commit
	hash, err := repo.ResolveRevision(plumbing.Revision(commit))
	if err != nil {
		return "", foundationerrors.WrapError(err, foundationerrors.CategoryGit, "failed to resolve commit").WithSeverity(foundationerrors.SeverityError).WithContext("commit", commit).Build()
	}

	commitObj, err := repo.CommitObject(*hash)
	if err != nil {
		return "", foundationerrors.WrapError(err, foundationerrors.CategoryGit, "failed to get commit object").WithSeverity(foundationerrors.SeverityError).WithContext("commit_hash", hash.String()).Build()
	}

	tree, err := commitObj.Tree()
	if err != nil {
		return "", foundationerrors.WrapError(err, foundationerrors.CategoryGit, "failed to get commit tree").WithSeverity(foundationerrors.SeverityError).Build()
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
				return "", foundationerrors.WrapError(err, foundationerrors.CategoryGit, "failed to hash tree").WithSeverity(foundationerrors.SeverityError).Build()
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
				return "", foundationerrors.WrapError(err, foundationerrors.CategoryGit, "failed to hash subtree").WithSeverity(foundationerrors.SeverityError).WithContext("path", path).Build()
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
			err := foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to stat file").WithSeverity(foundationerrors.SeverityError).WithContext("path", fullPath).Build()
			return "", err
		}

		if info.IsDir() {
			// Walk directory
			if err := hashDirectory(repoPath, fullPath, &fileHashes); err != nil {
				return "", err
			}
		} else {
			// Single file
			if err := hashSingleFile(repoPath, fullPath, &fileHashes); err != nil {
				return "", err
			}
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

// hashDirectory walks a directory and hashes all non-hidden files.
func hashDirectory(repoPath, dirPath string, fileHashes *[]string) error {
	err := filepath.Walk(dirPath, func(p string, info os.FileInfo, err error) error {
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
			return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to read file").WithSeverity(foundationerrors.SeverityError).WithContext("path", p).Build()
		}

		h := sha256.Sum256(content)
		relPath, _ := filepath.Rel(repoPath, p)
		*fileHashes = append(*fileHashes, fmt.Sprintf("%s:%s", relPath, hex.EncodeToString(h[:])))
		return nil
	})
	if err != nil {
		return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to walk directory").WithSeverity(foundationerrors.SeverityError).WithContext("directory", dirPath).Build()
	}
	return nil
}

// hashSingleFile hashes a single file and adds it to the hash list.
func hashSingleFile(repoPath, filePath string, fileHashes *[]string) error {
	// #nosec G304 - filePath is validated and within controlled directory
	content, err := os.ReadFile(filePath)
	if err != nil {
		return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to read file").WithSeverity(foundationerrors.SeverityError).WithContext("file_path", filePath).Build()
	}

	h := sha256.Sum256(content)
	relPath, _ := filepath.Rel(repoPath, filePath)
	*fileHashes = append(*fileHashes, fmt.Sprintf("%s:%s", relPath, hex.EncodeToString(h[:])))
	return nil
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
