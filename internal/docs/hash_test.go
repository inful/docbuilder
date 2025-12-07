package docs

import (
	"testing"
)

func TestComputeDocsHashConsistency(t *testing.T) {
	docFiles := []DocFile{
		{
			Path:         "/repo/docs/readme.md",
			RelativePath: "readme.md",
			Repository:   "test-repo",
			Section:      "docs",
			Content:      []byte("# Documentation"),
			Metadata:     map[string]string{"key": "value"},
		},
		{
			Path:         "/repo/docs/guide.md",
			RelativePath: "guide.md",
			Repository:   "test-repo",
			Section:      "docs",
			Content:      []byte("# Guide"),
		},
	}

	// Compute hash twice - should be identical
	hash1, err := ComputeDocsHash(docFiles)
	if err != nil {
		t.Fatalf("ComputeDocsHash failed: %v", err)
	}

	hash2, err := ComputeDocsHash(docFiles)
	if err != nil {
		t.Fatalf("ComputeDocsHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hash not consistent: %s != %s", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-char SHA256 hash, got %d chars", len(hash1))
	}
}

func TestComputeDocsHashOrderIndependent(t *testing.T) {
	doc1 := DocFile{
		Path:       "/repo/docs/a.md",
		Repository: "test-repo",
		Content:    []byte("Content A"),
	}

	doc2 := DocFile{
		Path:       "/repo/docs/b.md",
		Repository: "test-repo",
		Content:    []byte("Content B"),
	}

	// Different order, same files
	hash1, _ := ComputeDocsHash([]DocFile{doc1, doc2})
	hash2, _ := ComputeDocsHash([]DocFile{doc2, doc1})

	if hash1 != hash2 {
		t.Error("Hash should be order-independent (after sorting)")
	}
}

func TestComputeDocsHashChangesWithContent(t *testing.T) {
	docFiles1 := []DocFile{
		{
			Path:       "/repo/docs/readme.md",
			Repository: "test-repo",
			Content:    []byte("Version 1"),
		},
	}

	docFiles2 := []DocFile{
		{
			Path:       "/repo/docs/readme.md",
			Repository: "test-repo",
			Content:    []byte("Version 2"),
		},
	}

	hash1, _ := ComputeDocsHash(docFiles1)
	hash2, _ := ComputeDocsHash(docFiles2)

	if hash1 == hash2 {
		t.Error("Hash should change when content changes")
	}
}

func TestComputeDocsHashChangesWithFileCount(t *testing.T) {
	oneFile := []DocFile{
		{Path: "/repo/docs/a.md", Repository: "test-repo", Content: []byte("A")},
	}

	twoFiles := []DocFile{
		{Path: "/repo/docs/a.md", Repository: "test-repo", Content: []byte("A")},
		{Path: "/repo/docs/b.md", Repository: "test-repo", Content: []byte("B")},
	}

	hash1, _ := ComputeDocsHash(oneFile)
	hash2, _ := ComputeDocsHash(twoFiles)

	if hash1 == hash2 {
		t.Error("Hash should change when file count changes")
	}
}

func TestComputeDocsHashEmptySet(t *testing.T) {
	hash, err := ComputeDocsHash([]DocFile{})
	if err != nil {
		t.Fatalf("ComputeDocsHash failed on empty set: %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("Expected 64-char hash for empty set, got %d", len(hash))
	}

	// Empty set should be consistent
	hash2, _ := ComputeDocsHash([]DocFile{})
	if hash != hash2 {
		t.Error("Empty set hash not consistent")
	}
}

func TestComputeDocsHashWithMetadata(t *testing.T) {
	doc1 := []DocFile{
		{
			Path:       "/repo/docs/a.md",
			Repository: "test-repo",
			Content:    []byte("Content"),
			Metadata:   map[string]string{"key1": "value1"},
		},
	}

	doc2 := []DocFile{
		{
			Path:       "/repo/docs/a.md",
			Repository: "test-repo",
			Content:    []byte("Content"),
			Metadata:   map[string]string{"key1": "value2"},
		},
	}

	hash1, _ := ComputeDocsHash(doc1)
	hash2, _ := ComputeDocsHash(doc2)

	if hash1 == hash2 {
		t.Error("Hash should change when metadata changes")
	}
}

func TestCreateDocsManifest(t *testing.T) {
	docFiles := []DocFile{
		{
			Path:         "/repo/docs/readme.md",
			RelativePath: "readme.md",
			Repository:   "test-repo",
			Section:      "docs",
			Content:      []byte("# Documentation"),
		},
		{
			Path:         "/repo/docs/guide.md",
			RelativePath: "guide.md",
			Repository:   "test-repo",
			Section:      "docs",
			Content:      []byte("# Guide"),
		},
	}

	manifest, err := CreateDocsManifest(docFiles)
	if err != nil {
		t.Fatalf("CreateDocsManifest failed: %v", err)
	}

	if manifest.FileCount() != 2 {
		t.Errorf("Expected 2 files, got %d", manifest.FileCount())
	}

	if manifest.Hash == "" {
		t.Error("Manifest hash should not be empty")
	}

	// Check file ordering (should be sorted)
	if manifest.Files[0].Path > manifest.Files[1].Path {
		t.Error("Files should be sorted by path")
	}
}

func TestDocsManifestJSON(t *testing.T) {
	docFiles := []DocFile{
		{
			Path:       "/repo/docs/readme.md",
			Repository: "test-repo",
			Content:    []byte("# Documentation"),
		},
	}

	manifest, err := CreateDocsManifest(docFiles)
	if err != nil {
		t.Fatalf("CreateDocsManifest failed: %v", err)
	}

	// Serialize
	jsonData, err := manifest.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON data should not be empty")
	}

	// Deserialize
	manifest2, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if manifest.Hash != manifest2.Hash {
		t.Error("Hash mismatch after JSON roundtrip")
	}

	if manifest.FileCount() != manifest2.FileCount() {
		t.Error("File count mismatch after JSON roundtrip")
	}
}

func TestDocsManifestGetFileByPath(t *testing.T) {
	docFiles := []DocFile{
		{Path: "/repo/docs/a.md", Repository: "test-repo", Content: []byte("A")},
		{Path: "/repo/docs/b.md", Repository: "test-repo", Content: []byte("B")},
	}

	manifest, _ := CreateDocsManifest(docFiles)

	file := manifest.GetFileByPath("/repo/docs/a.md")
	if file == nil {
		t.Fatal("File not found")
	}

	if file.Path != "/repo/docs/a.md" {
		t.Errorf("Wrong file returned: %s", file.Path)
	}

	// Non-existent file
	file = manifest.GetFileByPath("/nonexistent")
	if file != nil {
		t.Error("Should return nil for non-existent file")
	}
}

func TestDocsManifestFilterByRepository(t *testing.T) {
	docFiles := []DocFile{
		{Path: "/repo1/docs/a.md", Repository: "repo1", Content: []byte("A")},
		{Path: "/repo1/docs/b.md", Repository: "repo1", Content: []byte("B")},
		{Path: "/repo2/docs/c.md", Repository: "repo2", Content: []byte("C")},
	}

	manifest, _ := CreateDocsManifest(docFiles)

	repo1Files := manifest.FilterByRepository("repo1")
	if len(repo1Files) != 2 {
		t.Errorf("Expected 2 files for repo1, got %d", len(repo1Files))
	}

	repo2Files := manifest.FilterByRepository("repo2")
	if len(repo2Files) != 1 {
		t.Errorf("Expected 1 file for repo2, got %d", len(repo2Files))
	}

	repo3Files := manifest.FilterByRepository("repo3")
	if len(repo3Files) != 0 {
		t.Errorf("Expected 0 files for repo3, got %d", len(repo3Files))
	}
}

func TestComputeDocsHashMultipleRepos(t *testing.T) {
	docFiles := []DocFile{
		{Path: "/repo1/a.md", Repository: "repo1", Content: []byte("A")},
		{Path: "/repo2/b.md", Repository: "repo2", Content: []byte("B")},
	}

	hash1, _ := ComputeDocsHash(docFiles)

	// Swap repos - hash should still be consistent (sorted by repo)
	docFiles2 := []DocFile{
		{Path: "/repo2/b.md", Repository: "repo2", Content: []byte("B")},
		{Path: "/repo1/a.md", Repository: "repo1", Content: []byte("A")},
	}

	hash2, _ := ComputeDocsHash(docFiles2)

	if hash1 != hash2 {
		t.Error("Hash should be consistent across repository order")
	}
}
