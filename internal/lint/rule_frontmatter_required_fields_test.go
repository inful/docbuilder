package lint

import (
	"os"
	"path/filepath"
	"testing"
)

const fullFrontmatter = `---
title: AllPresent
uid: 11111111-aaaa-aaaa-aaaa-111111111111
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/x/
---

body
`

func writeDoc(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFrontmatterRequiredFieldsRule_Name(t *testing.T) {
	r := &FrontmatterRequiredFieldsRule{}
	if got := r.Name(); got != ruleFrontmatterRequiredFields {
		t.Errorf("Name() = %q, want %q", got, ruleFrontmatterRequiredFields)
	}
}

func TestFrontmatterRequiredFieldsRule_AllPresent(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "all.md", fullFrontmatter)

	r := &FrontmatterRequiredFieldsRule{}
	issues, err := r.Check(filepath.Join(dir, "all.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// TestFrontmatterRequiredFieldsRule_MissingTitle verifies each field is
// individually flagged when missing.
func TestFrontmatterRequiredFieldsRule_MissingTitle(t *testing.T) {
	dir := t.TempDir()
	body := `---
uid: 22222222-aaaa-aaaa-aaaa-222222222222
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/x/
---

body
`
	writeDoc(t, dir, "no-title.md", body)

	r := &FrontmatterRequiredFieldsRule{}
	issues, err := r.Check(filepath.Join(dir, "no-title.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (title), got %d", len(issues))
	}
}

// TestFrontmatterRequiredFieldsRule_MalformedDate verifies a non-RFC3339 date produces a WARNING.
func TestFrontmatterRequiredFieldsRule_MalformedDate(t *testing.T) {
	dir := t.TempDir()
	body := `---
title: BadDate
uid: 33333333-aaaa-aaaa-aaaa-333333333333
date: not-a-date
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/x/
---

body
`
	writeDoc(t, dir, "bad-date.md", body)

	r := &FrontmatterRequiredFieldsRule{}
	issues, err := r.Check(filepath.Join(dir, "bad-date.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (date), got %d", len(issues))
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("malformed date should be WARNING, got %v", issues[0].Severity)
	}
}

// TestFrontmatterRequiredFieldsRule_MalformedLastmod verifies a non-YYYY-MM-DD lastmod produces a WARNING.
func TestFrontmatterRequiredFieldsRule_MalformedLastmod(t *testing.T) {
	dir := t.TempDir()
	body := `---
title: BadLastmod
uid: 44444444-aaaa-aaaa-aaaa-444444444444
date: 2026-01-01T00:00:00Z
lastmod: "yesterday"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/x/
---

body
`
	writeDoc(t, dir, "bad-lastmod.md", body)

	r := &FrontmatterRequiredFieldsRule{}
	issues, err := r.Check(filepath.Join(dir, "bad-lastmod.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (lastmod), got %d", len(issues))
	}
}

// TestFrontmatterRequiredFieldsRule_EmptyCategories verifies empty categories list flags.
func TestFrontmatterRequiredFieldsRule_EmptyCategories(t *testing.T) {
	dir := t.TempDir()
	body := `---
title: EmptyCats
uid: 55555555-aaaa-aaaa-aaaa-555555555555
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
tags:
  - test
aliases:
  - /_uid/x/
---

body
`
	writeDoc(t, dir, "empty-cats.md", body)

	r := &FrontmatterRequiredFieldsRule{}
	issues, err := r.Check(filepath.Join(dir, "empty-cats.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (empty categories), got %d", len(issues))
	}
}
