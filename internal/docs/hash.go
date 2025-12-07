package docs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// DocsManifest represents a collection of discovered documentation files.
type DocsManifest struct {
	Files []DocFileManifest `json:"files"`
	Hash  string            `json:"hash"`
}

// DocFileManifest represents a single documentation file in the manifest.
type DocFileManifest struct {
	Path         string            `json:"path"`
	RelativePath string            `json:"relative_path"`
	Repository   string            `json:"repository"`
	Forge        string            `json:"forge,omitempty"`
	Section      string            `json:"section"`
	ContentHash  string            `json:"content_hash"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ComputeDocsHash computes a deterministic hash for a set of documentation files.
// The hash is based on:
// - File paths (relative and absolute)
// - Content hashes
// - Repository/forge information
// - Metadata
//
// This enables detection of changes in the documentation set.
func ComputeDocsHash(docFiles []DocFile) (string, error) {
	if len(docFiles) == 0 {
		// Empty set has a known hash
		h := sha256.Sum256([]byte("empty-docs-set"))
		return hex.EncodeToString(h[:]), nil
	}

	// Convert to manifest entries
	var entries []DocFileManifest
	for _, df := range docFiles {
		// Compute content hash
		contentHash := ""
		if len(df.Content) > 0 {
			h := sha256.Sum256(df.Content)
			contentHash = hex.EncodeToString(h[:])
		}

		entries = append(entries, DocFileManifest{
			Path:         df.Path,
			RelativePath: df.RelativePath,
			Repository:   df.Repository,
			Forge:        df.Forge,
			Section:      df.Section,
			ContentHash:  contentHash,
			Metadata:     df.Metadata,
		})
	}

	// Sort for deterministic ordering
	sort.Slice(entries, func(i, j int) bool {
		// Primary sort: repository
		if entries[i].Repository != entries[j].Repository {
			return entries[i].Repository < entries[j].Repository
		}
		// Secondary sort: path
		return entries[i].Path < entries[j].Path
	})

	// Compute hash from sorted entries
	h := sha256.New()
	for _, entry := range entries {
		// Include all fields in hash
		data := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			entry.Path,
			entry.RelativePath,
			entry.Repository,
			entry.Forge,
			entry.Section,
			entry.ContentHash,
		)
		h.Write([]byte(data))
		h.Write([]byte("\n"))

		// Include metadata
		if len(entry.Metadata) > 0 {
			// Sort metadata keys for determinism
			var keys []string
			for k := range entry.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				h.Write([]byte(fmt.Sprintf("%s=%s\n", k, entry.Metadata[k])))
			}
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// CreateDocsManifest creates a manifest from documentation files.
func CreateDocsManifest(docFiles []DocFile) (*DocsManifest, error) {
	hash, err := ComputeDocsHash(docFiles)
	if err != nil {
		return nil, err
	}

	var entries []DocFileManifest
	for _, df := range docFiles {
		contentHash := ""
		if len(df.Content) > 0 {
			h := sha256.Sum256(df.Content)
			contentHash = hex.EncodeToString(h[:])
		}

		entries = append(entries, DocFileManifest{
			Path:         df.Path,
			RelativePath: df.RelativePath,
			Repository:   df.Repository,
			Forge:        df.Forge,
			Section:      df.Section,
			ContentHash:  contentHash,
			Metadata:     df.Metadata,
		})
	}

	// Sort for consistency
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Repository != entries[j].Repository {
			return entries[i].Repository < entries[j].Repository
		}
		return entries[i].Path < entries[j].Path
	})

	return &DocsManifest{
		Files: entries,
		Hash:  hash,
	}, nil
}

// ToJSON serializes the manifest to JSON.
func (m *DocsManifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// FromJSON deserializes a manifest from JSON.
func FromJSON(data []byte) (*DocsManifest, error) {
	var manifest DocsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return &manifest, nil
}

// GetFileByPath finds a file in the manifest by its path.
func (m *DocsManifest) GetFileByPath(path string) *DocFileManifest {
	for i := range m.Files {
		if m.Files[i].Path == path {
			return &m.Files[i]
		}
	}
	return nil
}

// FilterByRepository returns files for a specific repository.
func (m *DocsManifest) FilterByRepository(repo string) []DocFileManifest {
	var result []DocFileManifest
	for _, f := range m.Files {
		if f.Repository == repo {
			result = append(result, f)
		}
	}
	return result
}

// FileCount returns the number of files in the manifest.
func (m *DocsManifest) FileCount() int {
	return len(m.Files)
}
