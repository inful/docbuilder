package lint

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var errStop = errors.New("stop iteration")

// healBrokenLinks attempts to fix links that were broken by external renames/moves
// using Git history to find where the files went.
func (f *Fixer) healBrokenLinks(fixResult *FixResult, _ map[string]struct{}, root string) {
	if f.gitRepo == nil {
		return
	}

	brokenLinks, err := detectBrokenLinks(root)
	if err != nil || len(brokenLinks) == 0 {
		return
	}

	for _, bl := range brokenLinks {
		// Resolve the absolute path of the broken link target
		absTarget, err := resolveRelativePath(bl.SourceFile, bl.Target)
		if err != nil {
			continue
		}

		// Attempt to find where absTarget went
		newPath, err := f.findMovedFileInHistory(absTarget, root)
		if err == nil && newPath != "" {
			// Calculate relative link from SourceFile to newPath
			relLink, err := filepath.Rel(filepath.Dir(bl.SourceFile), newPath)
			if err == nil {
				// Normalize for Markdown
				relLink = filepath.ToSlash(relLink)

				err = f.updateLinkInFile(bl.SourceFile, bl.Target, relLink)
				if err == nil {
					fixResult.AddLinkUpdate(bl.SourceFile, bl.Target, relLink)
				}
			}
		}
	}
}

// updateLinkInFile replaces an old link with a new one in a file.
func (f *Fixer) updateLinkInFile(sourcePath, oldLink, newLink string) error {
	// #nosec G304 -- sourcePath is from discovery walkFiles, not user input
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	newContent := strings.ReplaceAll(string(content), "("+oldLink+")", "("+newLink+")")
	newContent = strings.ReplaceAll(newContent, "["+oldLink+"]", "["+newLink+"]")

	if newContent == string(content) {
		return errors.New("link not found in file")
	}

	return os.WriteFile(sourcePath, []byte(newContent), 0o600)
}

// findMovedFileInHistory searches Git logs to find a rename or move of the target path.
func (f *Fixer) findMovedFileInHistory(targetPath string, root string) (string, error) {
	ref, err := f.gitRepo.Head()
	if err != nil {
		return "", err
	}

	cIter, err := f.gitRepo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return "", err
	}

	// We only look back a few commits to avoid heavy processing
	const maxCommits = 10
	count := 0

	relativeTarget, err := filepath.Rel(root, targetPath)
	if err != nil {
		return "", err
	}
	relativeTarget = filepath.ToSlash(relativeTarget)

	var foundPath string
	err = cIter.ForEach(func(c *object.Commit) error {
		if count >= maxCommits {
			return errStop
		}
		count++

		stats, sErr := c.Stats()
		if sErr != nil {
			return nil // Skip this commit on error
		}

		// Check if targetPath was deleted or renamed in this commit
		deleted := false
		for _, stat := range stats {
			// Handle regular deletion
			if stat.Name == relativeTarget && stat.Addition == 0 && stat.Deletion > 0 {
				deleted = true
				break
			}

			// Handle git-detected rename "old => new"
			if strings.HasPrefix(stat.Name, relativeTarget+" => ") {
				parts := strings.Split(stat.Name, " => ")
				if len(parts) == 2 {
					foundPath = filepath.Join(root, parts[1])
					return errStop
				}
			}
		}

		if deleted {
			foundPath = f.findSameContentFile(c, relativeTarget, root)
			if foundPath != "" {
				return errStop
			}
		}

		return nil
	})

	if err != nil && !errors.Is(err, errStop) {
		return "", err
	}

	return foundPath, nil
}

// findSameContentFile searches for a file with the same content as the deleted one in the same commit.
func (f *Fixer) findSameContentFile(c *object.Commit, relativeTarget, root string) string {
	// Get tree of this commit
	tree, err := c.Tree()
	if err != nil {
		return ""
	}

	// We need to find the blob hash of the targetPath in the PREVIOUS commit
	if c.NumParents() == 0 {
		return ""
	}
	parent, err := c.Parent(0)
	if err != nil {
		return ""
	}
	parentTree, err := parent.Tree()
	if err != nil {
		return ""
	}

	entry, err := parentTree.FindEntry(relativeTarget)
	if err != nil {
		return ""
	}

	targetHash := entry.Hash

	var foundPath string
	// Search the current commit's tree for the same hash
	_ = tree.Files().ForEach(func(file *object.File) error {
		if file.Hash == targetHash && file.Name != relativeTarget {
			foundPath = filepath.Join(root, file.Name)
			return errStop
		}
		return nil
	})

	return foundPath
}
