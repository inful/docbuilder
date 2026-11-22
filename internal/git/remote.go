package git

import (
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// RemoteReference represents a Git reference from a remote repository.
type RemoteReference struct {
	Name      string    // Short name (e.g., "main", "v1.0.0")
	RefName   string    // Full reference name (e.g., "refs/heads/main", "refs/tags/v1.0.0")
	Hash      string    // Commit SHA
	CreatedAt time.Time // Creation time (approximate)
}

// ListRemoteReferencesWithAuth lists remote references with explicit authentication.
func (c *Client) ListRemoteReferencesWithAuth(repoURL string, authConfig *appcfg.AuthConfig) ([]*RemoteReference, error) {
	// Create remote with authentication
	remoteConfig := &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	}

	remote := git.NewRemote(memory.NewStorage(), remoteConfig)

	// Setup authentication
	listOptions := &git.ListOptions{}
	if authConfig != nil {
		auth, err := c.getAuth(authConfig)
		if err != nil {
			return nil, GitError("failed to setup authentication").
				WithCause(err).
				Build()
		}
		listOptions.Auth = auth
	}

	// List references
	refs, err := remote.List(listOptions)
	if err != nil {
		return nil, ClassifyGitError(err, "list", repoURL)
	}

	remoteRefs := make([]*RemoteReference, 0, len(refs))
	for _, ref := range refs {
		refName := ref.Name().String()

		// Skip symbolic references
		if ref.Type() == plumbing.SymbolicReference {
			continue
		}

		var shortName string
		var include bool

		// Extract branch names
		if after, ok := strings.CutPrefix(refName, "refs/heads/"); ok {
			shortName = after
			include = true
		} else if after, ok := strings.CutPrefix(refName, "refs/tags/"); ok {
			shortName = after
			include = true
		}

		if !include {
			continue
		}

		remoteRef := &RemoteReference{
			Name:      shortName,
			RefName:   refName,
			Hash:      ref.Hash().String(),
			CreatedAt: time.Now(), // We don't have actual creation time without cloning
		}

		remoteRefs = append(remoteRefs, remoteRef)
	}

	return remoteRefs, nil
}
