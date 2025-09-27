package git

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// Client handles Git operations
type Client struct {
	workspaceDir string
	buildCfg     *appcfg.BuildConfig // optional build config for strategy flags
	inRetry      bool                // internal guard to avoid nested retry wrapping
}

// NewClient creates a new Git client with the specified workspace directory
func NewClient(workspaceDir string) *Client { return &Client{workspaceDir: workspaceDir} }

// WithBuildConfig attaches build configuration to the client (fluent helper).
func (c *Client) WithBuildConfig(cfg *appcfg.BuildConfig) *Client { c.buildCfg = cfg; return c }

// CloneRepository clones a repository to the workspace
func (c *Client) CloneRepository(repo appcfg.Repository) (string, error) {
	// Wrap core operation with retry if build config present (and not already inside a retry context).
	if c.inRetry {
		return c.cloneOnce(repo)
	}
	return c.withRetry("clone", repo.Name, func() (string, error) {
		return c.cloneOnce(repo)
	})
}

func (c *Client) cloneOnce(repo appcfg.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)

	slog.Debug("Cloning repository", logfields.URL(repo.URL), logfields.Name(repo.Name), slog.String("branch", repo.Branch), logfields.Path(repoPath))

	// Remove existing directory if it exists
	if err := os.RemoveAll(repoPath); err != nil {
		return "", fmt.Errorf("failed to remove existing directory: %w", err)
	}

	// Create clone options
	cloneOptions := &git.CloneOptions{URL: repo.URL, Progress: os.Stdout}

	// Set branch if specified
 	if repo.Branch != "" {
 		cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo.Branch)
 		cloneOptions.SingleBranch = true
 	}
 	// Shallow clone depth if configured
 	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
 		cloneOptions.Depth = c.buildCfg.ShallowDepth
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

 	// Optional pruning of non-doc top-level directories
 	if c.buildCfg != nil && c.buildCfg.PruneNonDocPaths {
 		if err := c.pruneNonDocTopLevel(repoPath, repo); err != nil {
 			slog.Warn("prune non-doc paths failed", logfields.Name(repo.Name), slog.String("error", err.Error()))
 		}
 	}
 	return repoPath, nil
}

// UpdateRepository updates an existing repository or clones if it doesn't exist
func (c *Client) UpdateRepository(repo appcfg.Repository) (string, error) {
 	if c.inRetry {
 		return c.updateOnce(repo)
 	}
 	return c.withRetry("update", repo.Name, func() (string, error) {
 		return c.updateOnce(repo)
 	})
}

// updateOnce handles an update without wrapping it in retry logic.
func (c *Client) updateOnce(repo appcfg.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	// If repository missing, perform clone regardless of strategy used.
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		slog.Debug("Repository missing, cloning", logfields.Name(repo.Name))
		return c.cloneOnce(repo)
	}
	return c.updateExistingRepo(repoPath, repo)
}

// withRetry wraps clone/update operations with retry based on build configuration.
func (c *Client) withRetry(op, repoName string, fn func() (string, error)) (string, error) {
	if c.buildCfg == nil || c.buildCfg.MaxRetries <= 0 { // no retry configured
		return fn()
	}
	initial, _ := time.ParseDuration(c.buildCfg.RetryInitialDelay)
	if initial <= 0 { initial = 500 * time.Millisecond }
	maxDelay, _ := time.ParseDuration(c.buildCfg.RetryMaxDelay)
	if maxDelay <= 0 { maxDelay = 10 * time.Second }
	var lastErr error
	for attempt := 0; attempt <= c.buildCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("retrying git operation", slog.String("operation", op), logfields.Name(repoName), slog.Int("attempt", attempt))
		}
		c.inRetry = true
		path, err := fn()
		c.inRetry = false
		if err == nil { return path, nil }
		lastErr = err
		if isPermanentGitError(err) { // do not retry permanent errors
			slog.Error("permanent git error", slog.String("operation", op), logfields.Name(repoName), slog.String("error", err.Error()))
			return "", err
		}
		if attempt == c.buildCfg.MaxRetries { break }
		delay := computeBackoffDelay(string(c.buildCfg.RetryBackoff), attempt, initial, maxDelay)
		time.Sleep(delay)
	}
	return "", fmt.Errorf("git %s failed after retries: %w", op, lastErr)
}

// computeBackoffDelay returns a delay based on backoff strategy.
func computeBackoffDelay(strategy string, attempt int, initial, max time.Duration) time.Duration {
	if attempt <= 0 { return initial }
	switch strings.ToLower(strategy) {
	case "linear":
		d := time.Duration(attempt+1) * initial
		if d > max { return max }
		return d
	case "exponential":
		d := initial * (1 << attempt)
		if d > max { return max }
		return d
	case "fixed", "":
		fallthrough
	default:
		if initial > max { return max }
		return initial
	}
}

// isPermanentGitError heuristically classifies an error as non-retryable.
func isPermanentGitError(err error) bool {
	if err == nil { return false }
	msg := strings.ToLower(err.Error())
	// Authentication / authorization issues
	if strings.Contains(msg, "auth") || strings.Contains(msg, "permission") || strings.Contains(msg, "denied") {
		return true
	}
	// Repository not found or invalid reference
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such remote") || strings.Contains(msg, "invalid reference") {
		return true
	}
	// Unsupported protocol
	if strings.Contains(msg, "unsupported protocol") { return true }
	// Network timeouts & temporary network failures are retryable, so if it is a net error that is timeout return false.
	var nerr net.Error
	if errors.As(err, &nerr) { return !nerr.Timeout() }
	return false
}

// updateExistingRepo performs a fetch + fast-forward / hard reset based on configuration.
func (c *Client) updateExistingRepo(repoPath string, repo appcfg.Repository) (string, error) {
	repository, err := git.PlainOpen(repoPath)
	if err != nil { return "", fmt.Errorf("open repo: %w", err) }
	wt, err := repository.Worktree()
	if err != nil { return "", fmt.Errorf("worktree: %w", err) }

	depth := 0
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 { depth = c.buildCfg.ShallowDepth }
	fetchOpts := &git.FetchOptions{RemoteName: "origin", Tags: git.NoTags}
	if depth > 0 { fetchOpts.Depth = depth }
	// Force update all branches (so we can detect divergence)
	fetchOpts.RefSpecs = []ggitcfg.RefSpec{"+refs/heads/*:refs/remotes/origin/*"}

	if repo.Auth != nil {
		auth, aerr := c.getAuthentication(repo.Auth)
		if aerr != nil { return "", aerr }
		fetchOpts.Auth = auth
	}

	if err := repository.Fetch(fetchOpts); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("fetch: %w", err)
	}

	// Determine branch to operate on.
	branch := repo.Branch
	if branch == "" {
		// Try current HEAD
		if headRef, herr := repository.Head(); herr == nil && headRef.Name().IsBranch() {
			branch = headRef.Name().Short()
		} else {
			if def, derr := resolveRemoteDefaultBranch(repository); derr == nil { branch = def } else { branch = "main" }
		}
	}

	localBranchRef := plumbing.NewBranchReferenceName(branch)
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branch)

	remoteRef, err := repository.Reference(remoteBranchRef, true)
	if err != nil { return "", fmt.Errorf("remote ref: %w", err) }

	localRef, lerr := repository.Reference(localBranchRef, true)
	if lerr != nil { // create local branch at remote
		if err := wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Create: true, Force: true}); err != nil {
			return "", fmt.Errorf("checkout new branch: %w", err)
		}
		localRef, _ = repository.Reference(localBranchRef, true)
	} else {
		if err := wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Force: true}); err != nil {
			return "", fmt.Errorf("checkout existing branch: %w", err)
		}
	}

	// Determine relationship between local and remote
	fastForwardPossible, ffErr := isAncestor(repository, localRef.Hash(), remoteRef.Hash())
	if ffErr != nil { slog.Warn("ancestor check failed", slog.String("error", ffErr.Error())) }

	if fastForwardPossible {
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return "", fmt.Errorf("fast-forward reset: %w", err)
		}
	} else {
		// Diverged - decide strategy
		hardReset := c.buildCfg != nil && c.buildCfg.HardResetOnDiverge
		if hardReset {
			slog.Warn("diverged branch, hard resetting", logfields.Name(repo.Name), slog.String("branch", branch))
			if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
				return "", fmt.Errorf("hard reset: %w", err)
			}
		} else {
			return "", fmt.Errorf("local branch diverged from remote (enable hard_reset_on_diverge to override)")
		}
	}

	// Optionally clean untracked files
	if c.buildCfg != nil && c.buildCfg.CleanUntracked {
		if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
			slog.Warn("clean untracked failed", slog.String("error", err.Error()))
		}
	}

	// Prune after update if enabled
	if c.buildCfg != nil && c.buildCfg.PruneNonDocPaths {
		if err := c.pruneNonDocTopLevel(repoPath, repo); err != nil {
			slog.Warn("prune non-doc paths failed", logfields.Name(repo.Name), slog.String("error", err.Error()))
		}
	}

	// Log state
	if headRef, err := repository.Head(); err == nil {
		slog.Info("Repository updated", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("commit", headRef.Hash().String()[:8]))
	} else {
		slog.Info("Repository updated", logfields.Name(repo.Name), slog.String("branch", branch))
	}
	return repoPath, nil
}

// resolveRemoteDefaultBranch attempts to read origin/HEAD symbolic ref.
func resolveRemoteDefaultBranch(repo *git.Repository) (string, error) {
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), true)
	if err != nil { return "", err }
	target := ref.Target()
	if target == "" { return "", fmt.Errorf("origin/HEAD target empty") }
	return plumbing.ReferenceName(target).Short(), nil
}

// isAncestor returns true if commit a is an ancestor of commit b.
func isAncestor(repo *git.Repository, a, b plumbing.Hash) (bool, error) {
	if a == b { return true, nil }
	seen := map[plumbing.Hash]struct{}{}
	queue := []plumbing.Hash{b}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if h == a { return true, nil }
		if _, ok := seen[h]; ok { continue }
		seen[h] = struct{}{}
		commit, err := repo.CommitObject(h)
		if err != nil { return false, err }
		for _, p := range commit.ParentHashes { queue = append(queue, p) }
	}
	return false, nil
}

// pruneNonDocTopLevel removes top-level entries not related to configured doc paths
// while respecting allow/deny lists (glob aware). The repo.Paths entries may contain
// nested segments (e.g. docs/api) or extraneous prefixes like ./docs.
func (c *Client) pruneNonDocTopLevel(repoPath string, repo appcfg.Repository) error {
	if c.buildCfg == nil || !c.buildCfg.PruneNonDocPaths { return nil }

	// Build a set of top-level doc roots derived from repo.Paths.
	docRoots := map[string]struct{}{}
	for _, p := range repo.Paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" { continue }
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimPrefix(p, "/")
		parts := strings.Split(p, "/")
		if len(parts) > 0 && parts[0] != "" { docRoots[parts[0]] = struct{}{} }
	}

	allowPatterns := c.buildCfg.PruneAllow
	denyPatterns := c.buildCfg.PruneDeny

	entries, err := os.ReadDir(repoPath)
	if err != nil { return fmt.Errorf("readdir: %w", err) }

	matchesAny := func(name string, patterns []string) bool {
		for _, pat := range patterns {
			if pat == "" { continue }
			if strings.EqualFold(pat, name) { return true }
			if ok, _ := filepath.Match(pat, name); ok { return true }
		}
		return false
	}

	for _, ent := range entries {
		name := ent.Name()
		if name == ".git" { continue } // Always preserve .git directory
		_, isDocRoot := docRoots[name]
		if isDocRoot { continue }
		if matchesAny(name, denyPatterns) { // Deny has precedence
			if err := os.RemoveAll(filepath.Join(repoPath, name)); err != nil { return fmt.Errorf("remove denied %s: %w", name, err) }
			continue
		}
		if matchesAny(name, allowPatterns) { // explicitly allowed
			continue
		}
		// Remove everything else (files or directories)
		if err := os.RemoveAll(filepath.Join(repoPath, name)); err != nil { return fmt.Errorf("remove %s: %w", name, err) }
	}
	return nil
}

// getAuthentication creates authentication based on config
func (c *Client) getAuthentication(auth *appcfg.AuthConfig) (transport.AuthMethod, error) {
	switch auth.Type {
	case appcfg.AuthTypeNone, "":
		return nil, nil // No authentication needed for public repositories
	case appcfg.AuthTypeSSH:
		if auth.KeyPath == "" {
			auth.KeyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		}
		publicKeys, err := ssh.NewPublicKeysFromFile("git", auth.KeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", auth.KeyPath, err)
		}
		return publicKeys, nil
	case appcfg.AuthTypeToken:
		if auth.Token == "" {
			return nil, fmt.Errorf("token authentication requires a token")
		}
		return &http.BasicAuth{Username: "token", Password: auth.Token}, nil
	case appcfg.AuthTypeBasic:
		if auth.Username == "" || auth.Password == "" {
			return nil, fmt.Errorf("basic authentication requires username and password")
		}
		return &http.BasicAuth{Username: auth.Username, Password: auth.Password}, nil
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

