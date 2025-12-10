package incremental

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// ComputeRepoHashes computes content hashes for all cloned repositories.
// Returns a slice of RepoHash structs that can be used for cache checking.
func ComputeRepoHashes(repos []config.Repository, repoPaths map[string]string) ([]RepoHash, error) {
	var repoHashes []RepoHash

	for _, repo := range repos {
		repoPath, ok := repoPaths[repo.Name]
		if !ok {
			// Repository was skipped during cloning
			continue
		}

		// Compute hash for the repository using its configured paths
		paths := repo.Paths
		if len(paths) == 0 {
			paths = []string{"docs"} // default
		}

		// Use the current working directory state for hash computation
		hash, err := git.ComputeRepoHashFromWorkdir(repoPath, paths)
		if err != nil {
			return nil, fmt.Errorf("compute hash for %s: %w", repo.Name, err)
		}

		// Get current commit for this repo
		commit, err := git.ReadRepoHead(repoPath)
		if err != nil {
			return nil, fmt.Errorf("get commit for %s: %w", repo.Name, err)
		}

		repoHashes = append(repoHashes, RepoHash{
			Name:   repo.Name,
			Commit: commit,
			Hash:   hash,
		})
	}

	return repoHashes, nil
}

// ComputeSimpleBuildSignature creates a simplified build signature for cache checking
// without requiring a full BuildPlan (which is only used in Phase 3 pipeline).
func ComputeSimpleBuildSignature(cfg *config.Config, repoHashes []RepoHash) (*BuildSignature, error) {
	// Sort repos by name for determinism
	sorted := make([]RepoHash, len(repoHashes))
	copy(sorted, repoHashes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	// Create signature with essential fields
	sig := &BuildSignature{
		RepoHashes: sorted,
		Theme:      cfg.Hugo.Theme,
		ThemeVer:   "",         // Simple version doesn't track theme version
		Transforms: []string{}, // Simple version doesn't track transforms
		Metadata:   make(map[string]string),
	}

	// Compute config hash from essential config fields
	configData, err := json.Marshal(struct {
		Theme   string
		BaseURL string
		Title   string
	}{
		Theme:   cfg.Hugo.Theme,
		BaseURL: cfg.Hugo.BaseURL,
		Title:   cfg.Hugo.Title,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	sig.ConfigHash = fmt.Sprintf("%x", sha256.Sum256(configData))

	// Compute overall build hash
	sigData, err := json.Marshal(sig)
	if err != nil {
		return nil, fmt.Errorf("marshal signature: %w", err)
	}
	sig.BuildHash = hex.EncodeToString(sha256.New().Sum(sigData)[:])

	return sig, nil
}
