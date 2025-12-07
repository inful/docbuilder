// Package incremental provides functionality for incremental builds based on content hashing.
package incremental

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/pipeline"
)

// RepoHash represents a repository's content hash along with commit information.
type RepoHash struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
	Hash   string `json:"hash"` // content hash from git.ComputeRepoHash
}

// BuildSignature represents the complete signature of a build's inputs.
type BuildSignature struct {
	RepoHashes []RepoHash        `json:"repo_hashes"`
	Theme      string            `json:"theme"`
	ThemeVer   string            `json:"theme_version"`
	Transforms []string          `json:"transforms"`
	ConfigHash string            `json:"config_hash"`
	BuildHash  string            `json:"build_hash"` // computed hash of all above
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ComputeBuildSignature computes a deterministic hash for the entire build.
// The signature includes:
// - All repository hashes (commit + content)
// - Theme name and version
// - Transform names (sorted for determinism)
// - Config hash
//
// Two builds with identical signatures can safely reuse artifacts.
func ComputeBuildSignature(plan *pipeline.BuildPlan, repos []RepoHash) (*BuildSignature, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan cannot be nil")
	}

	// Compute config hash
	configHash, err := computeConfigHash(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to compute config hash: %w", err)
	}

	sig := &BuildSignature{
		RepoHashes: repos,
		Theme:      string(plan.ThemeFeatures.Name),
		ThemeVer:   "", // TODO: add theme version tracking
		Transforms: plan.TransformNames,
		ConfigHash: configHash,
		Metadata:   make(map[string]string),
	}

	// Sort repos by name for determinism
	sort.Slice(sig.RepoHashes, func(i, j int) bool {
		return sig.RepoHashes[i].Name < sig.RepoHashes[j].Name
	})

	// Sort transforms for determinism (already sorted in plan, but ensure it)
	sort.Strings(sig.Transforms)

	// Compute hash of all signature components
	hash, err := computeSignatureHash(sig)
	if err != nil {
		return nil, fmt.Errorf("failed to compute signature hash: %w", err)
	}

	sig.BuildHash = hash
	return sig, nil
}

// computeConfigHash computes a hash of the build configuration.
func computeConfigHash(plan *pipeline.BuildPlan) (string, error) {
	if plan.Config == nil {
		return "", nil
	}

	// Serialize config to JSON for hashing
	data, err := json.Marshal(plan.Config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// computeSignatureHash computes SHA256 hash of signature components.
func computeSignatureHash(sig *BuildSignature) (string, error) {
	// Create a normalized representation for hashing
	// Exclude BuildHash and Metadata from the hash computation
	normalized := struct {
		RepoHashes []RepoHash `json:"repo_hashes"`
		Theme      string     `json:"theme"`
		ThemeVer   string     `json:"theme_version"`
		Transforms []string   `json:"transforms"`
		ConfigHash string     `json:"config_hash"`
	}{
		RepoHashes: sig.RepoHashes,
		Theme:      sig.Theme,
		ThemeVer:   sig.ThemeVer,
		Transforms: sig.Transforms,
		ConfigHash: sig.ConfigHash,
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ToJSON serializes the signature to JSON.
func (s *BuildSignature) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// FromJSON deserializes a signature from JSON.
func FromJSON(data []byte) (*BuildSignature, error) {
	var sig BuildSignature
	if err := json.Unmarshal(data, &sig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal signature: %w", err)
	}
	return &sig, nil
}

// Equals checks if two signatures are equal (same BuildHash).
func (s *BuildSignature) Equals(other *BuildSignature) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.BuildHash == other.BuildHash
}
