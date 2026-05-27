package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectTaxonomiesFromContent(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "content")
	if err := os.MkdirAll(filepath.Join(contentDir, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}

	page1 := `---
title: First
tags:
  - alpha
  - Beta
categories:
  - Team
---

Body
`
	if err := os.WriteFile(filepath.Join(contentDir, "docs", "first.md"), []byte(page1), 0o600); err != nil {
		t.Fatalf("write first.md: %v", err)
	}

	page2 := `---
title: Second
tags: beta
categories: team
---

Body
`
	if err := os.WriteFile(filepath.Join(contentDir, "docs", "second.md"), []byte(page2), 0o600); err != nil {
		t.Fatalf("write second.md: %v", err)
	}

	tags, categories, err := CollectTaxonomiesFromContent(contentDir)
	if err != nil {
		t.Fatalf("CollectTaxonomiesFromContent: %v", err)
	}

	if len(tags) != 2 || tags[0] != "alpha" || tags[1] != "Beta" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if len(categories) != 1 || categories[0] != "Team" {
		t.Fatalf("unexpected categories: %#v", categories)
	}
}
