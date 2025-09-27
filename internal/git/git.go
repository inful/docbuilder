package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// Client handles Git operations
type Client struct {
	workspaceDir string
}

// NewClient creates a new Git client with the specified workspace directory
func NewClient(workspaceDir string) *Client {
	return &Client{
		workspaceDir: workspaceDir,
	}
}

// CloneRepository clones a repository to the workspace
func (c *Client) CloneRepository(repo config.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)

	slog.Debug("Cloning repository", logfields.URL(repo.URL), logfields.Name(repo.Name), slog.String("branch", repo.Branch), logfields.Path(repoPath))

	// Remove existing directory if it exists
	if err := os.RemoveAll(repoPath); err != nil {
		return "", fmt.Errorf("failed to remove existing directory: %w", err)
	}

	// Create clone options
	cloneOptions := &git.CloneOptions{
		URL:      repo.URL,
		Progress: os.Stdout,
	}

	// Set branch if specified
	if repo.Branch != "" {
		cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo.Branch)
		cloneOptions.SingleBranch = true
	}

	// Set authentication if provided
	if repo.Auth != nil {
		auth, err := c.getAuthentication(repo.Auth)
		if err != nil {
			return "", fmt.Errorf("failed to setup authentication: %w", err)
		}
		cloneOptions.Auth = auth
	}

	// Clone the repository
	repository, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		return "", fmt.Errorf("failed to clone repository %s: %w", repo.URL, err)
	}

	// Log clone success
	ref, err := repository.Head()
	if err == nil {
		slog.Info("Repository cloned successfully",
			logfields.Name(repo.Name),
			logfields.URL(repo.URL),
			slog.String("commit", ref.Hash().String()[:8]),
			logfields.Path(repoPath))
	} else {
		slog.Info("Repository cloned successfully",
			logfields.Name(repo.Name),
			logfields.URL(repo.URL),
			logfields.Path(repoPath))
	}

	return repoPath, nil
}

// UpdateRepository updates an existing repository or clones if it doesn't exist
func (c *Client) UpdateRepository(repo config.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		slog.Debug("Updating existing repository", logfields.Name(repo.Name), logfields.Path(repoPath))
		return c.updateExistingRepo(repoPath, repo)
	}

	// Repository doesn't exist, clone it
	slog.Debug("Repository doesn't exist, cloning", logfields.Name(repo.Name))
	return c.CloneRepository(repo)
}

// updateExistingRepo updates an existing Git repository
func (c *Client) updateExistingRepo(repoPath string, repo config.Repository) (string, error) {
	// Open the existing repository
	repository, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	// Get worktree
	worktree, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Setup pull options
	pullOptions := &git.PullOptions{
		RemoteName: "origin",
	}

	// Set authentication if provided
	if repo.Auth != nil {
		auth, err := c.getAuthentication(repo.Auth)
		if err != nil {
			return "", fmt.Errorf("failed to setup authentication: %w", err)
		}
		pullOptions.Auth = auth
	}

	// Pull latest changes
	err = worktree.Pull(pullOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("failed to pull repository %s: %w", repo.URL, err)
	}

	// Log update result
	if err == git.NoErrAlreadyUpToDate {
		slog.Info("Repository already up to date", logfields.Name(repo.Name))
	} else {
		ref, _ := repository.Head()
		slog.Info("Repository updated successfully",
			logfields.Name(repo.Name),
			slog.String("commit", ref.Hash().String()[:8]))
	}

	return repoPath, nil
}

// getAuthentication creates authentication based on config
func (c *Client) getAuthentication(auth *config.AuthConfig) (transport.AuthMethod, error) {
	switch auth.Type {
	case "none", "":
		return nil, nil // No authentication needed for public repositories

	case "ssh":
		if auth.KeyPath == "" {
			auth.KeyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		}

		publicKeys, err := ssh.NewPublicKeysFromFile("git", auth.KeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", auth.KeyPath, err)
		}
		return publicKeys, nil

	case "token":
		if auth.Token == "" {
			return nil, fmt.Errorf("token authentication requires a token")
		}
		return &http.BasicAuth{
			Username: "token", // GitHub/GitLab use "token" as username
			Password: auth.Token,
		}, nil

	case "basic":
		if auth.Username == "" || auth.Password == "" {
			return nil, fmt.Errorf("basic authentication requires username and password")
		}
		return &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Password,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", auth.Type)
	}
}

// EnsureWorkspace creates the workspace directory if it doesn't exist
func (c *Client) EnsureWorkspace() error {
	if err := os.MkdirAll(c.workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	return nil
}

// CleanWorkspace removes all contents from the workspace directory
func (c *Client) CleanWorkspace() error {
	entries, err := os.ReadDir(c.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clean
		}
		return fmt.Errorf("failed to read workspace directory: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(c.workspaceDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	slog.Info("Workspace cleaned", logfields.Path(c.workspaceDir))
	return nil
}

// CheckDocIgnore checks if a repository has a .docignore file in its root
func (c *Client) CheckDocIgnore(repoPath string) (bool, error) {
	docIgnorePath := filepath.Join(repoPath, ".docignore")

	if _, err := os.Stat(docIgnorePath); err == nil {
		slog.Debug("Found .docignore file", logfields.Path(docIgnorePath))
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, fmt.Errorf("failed to check .docignore file: %w", err)
	}
}
