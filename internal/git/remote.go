package git

import (
	"fmt"
	"strings"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

// RemoteReference represents a Git reference from a remote repository
type RemoteReference struct {
	Name      string    // Short name (e.g., "main", "v1.0.0")
	RefName   string    // Full reference name (e.g., "refs/heads/main", "refs/tags/v1.0.0")
	Hash      string    // Commit SHA
	CreatedAt time.Time // Creation time (approximate)
}

// ListRemoteReferences lists all branches and tags from a remote repository
func (c *Client) ListRemoteReferences(repoURL string) ([]*RemoteReference, error) {
	// Create remote with authentication if needed
	remoteConfig := &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	}

	remote := git.NewRemote(memory.NewStorage(), remoteConfig)

	// Setup authentication - try to find a matching repository config
	var auth interface{}
	if c.buildCfg != nil {
		// Note: We don't have direct access to repository auth config here
		// This is a limitation - ideally we'd pass auth info through the call
		// For now, we'll attempt without auth and let it fail if needed
		auth = nil
	}

	// List references
	listOptions := &git.ListOptions{}
	if auth != nil {
		// listOptions.Auth = auth.(transport.AuthMethod)
		// Currently not setting auth - will be added in next iteration
	}

	refs, err := remote.List(listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote references: %w", err)
	}

	var remoteRefs []*RemoteReference
	for _, ref := range refs {
		refName := ref.Name().String()

		// Skip symbolic references
		if ref.Type() == plumbing.SymbolicReference {
			continue
		}

		var shortName string
		var include bool

		// Extract branch names
		if strings.HasPrefix(refName, "refs/heads/") {
			shortName = strings.TrimPrefix(refName, "refs/heads/")
			include = true
		} else if strings.HasPrefix(refName, "refs/tags/") {
			shortName = strings.TrimPrefix(refName, "refs/tags/")
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

// ListRemoteReferencesWithAuth lists remote references with explicit authentication
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
		auth, err := c.getAuthentication(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to setup authentication: %w", err)
		}
		listOptions.Auth = auth
	}

	// List references
	refs, err := remote.List(listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote references: %w", err)
	}

	var remoteRefs []*RemoteReference
	for _, ref := range refs {
		refName := ref.Name().String()

		// Skip symbolic references
		if ref.Type() == plumbing.SymbolicReference {
			continue
		}

		var shortName string
		var include bool

		// Extract branch names
		if strings.HasPrefix(refName, "refs/heads/") {
			shortName = strings.TrimPrefix(refName, "refs/heads/")
			include = true
		} else if strings.HasPrefix(refName, "refs/tags/") {
			shortName = strings.TrimPrefix(refName, "refs/tags/")
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
