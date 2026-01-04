package git

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// RemoteHeadCache stores the last known remote HEAD for repositories
// to avoid unnecessary fetch operations when remote hasn't changed.
type RemoteHeadCache struct {
	mu      sync.RWMutex
	entries map[string]*RemoteHeadEntry
	path    string
}

// RemoteHeadEntry represents a cached remote HEAD reference.
type RemoteHeadEntry struct {
	URL       string    `json:"url"`
	Branch    string    `json:"branch"`
	CommitSHA string    `json:"commit_sha"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewRemoteHeadCache creates a new remote HEAD cache.
// If cacheDir is empty, caching is disabled.
func NewRemoteHeadCache(cacheDir string) (*RemoteHeadCache, error) {
	cache := &RemoteHeadCache{
		entries: make(map[string]*RemoteHeadEntry),
	}

	if cacheDir == "" {
		return cache, nil // No persistence
	}

	cache.path = filepath.Join(cacheDir, "remote-heads.json")

	// Load existing cache if present
	if err := cache.load(); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to load remote HEAD cache", "error", err)
	}

	return cache, nil
}

// GetRemoteHead performs a lightweight ls-remote to check the current remote HEAD
// without fetching the entire repository. Returns the commit SHA or error.
func (c *Client) GetRemoteHead(repo appcfg.Repository, branch string) (string, error) {
	if branch == "" {
		branch = "main" // default fallback
	}

	rem := git.NewRemote(nil, &ggitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{repo.URL},
	})

	var auth transport.AuthMethod
	if repo.Auth != nil {
		a, err := c.getAuth(repo.Auth)
		if err != nil {
			return "", fmt.Errorf("authentication: %w", err)
		}
		auth = a
	}

	listOpts := &git.ListOptions{}
	if auth != nil {
		listOpts.Auth = auth
	}

	refs, err := rem.List(listOpts)
	if err != nil {
		return "", fmt.Errorf("ls-remote: %w", err)
	}

	// Look for the specific branch
	branchRef := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == branchRef {
			return ref.Hash().String(), nil
		}
	}

	// Fallback: look for refs/heads/<branch>
	headRef := plumbing.NewRemoteReferenceName("origin", branch)
	for _, ref := range refs {
		if ref.Name() == headRef {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("branch %s not found on remote", branch)
}

// CheckRemoteChanged checks if remote HEAD has changed since last fetch.
// Returns: changed (true if fetch needed), currentSHA, error.
func (c *Client) CheckRemoteChanged(cache *RemoteHeadCache, repo appcfg.Repository, branch string) (bool, string, error) {
	if cache == nil {
		return true, "", nil // No cache, assume changed
	}

	// Get current remote HEAD
	currentSHA, err := c.GetRemoteHead(repo, branch)
	if err != nil {
		slog.Debug("Failed to check remote HEAD, will fetch",
			logfields.Name(repo.Name),
			"error", err.Error())
		return true, "", nil // On error, fetch anyway
	}

	// Check cache
	cached := cache.Get(repo.URL, branch)
	if cached == nil {
		slog.Debug("No cached remote HEAD, will fetch", logfields.Name(repo.Name))
		return true, currentSHA, nil
	}

	if cached.CommitSHA != currentSHA {
		slog.Info("Remote HEAD changed",
			logfields.Name(repo.Name),
			"old", cached.CommitSHA[:8],
			"new", currentSHA[:8])
		return true, currentSHA, nil
	}

	slog.Info("Remote HEAD unchanged, skipping fetch",
		logfields.Name(repo.Name),
		slog.String("commit", currentSHA[:8]))
	return false, currentSHA, nil
}

// Get retrieves a cached entry.
func (c *RemoteHeadCache) Get(url, branch string) *RemoteHeadEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(url, branch)
	return c.entries[key]
}

// Set updates the cache with a new remote HEAD.
func (c *RemoteHeadCache) Set(url, branch, commitSHA string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(url, branch)
	c.entries[key] = &RemoteHeadEntry{
		URL:       url,
		Branch:    branch,
		CommitSHA: commitSHA,
		UpdatedAt: time.Now(),
	}
}

// Save persists the cache to disk.
func (c *RemoteHeadCache) Save() error {
	if c.path == "" {
		return nil // No persistence
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.path), 0o750); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}

	return nil
}

// load reads the cache from disk.
func (c *RemoteHeadCache) load() error {
	if c.path == "" {
		return nil
	}

	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := json.Unmarshal(data, &c.entries); err != nil {
		return fmt.Errorf("unmarshal cache: %w", err)
	}

	return nil
}

func cacheKey(url, branch string) string {
	return fmt.Sprintf("%s:%s", url, branch)
}
