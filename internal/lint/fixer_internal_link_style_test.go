package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixer_InternalLinkStyleAppendMd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 66666666-aaaa-aaaa-aaaa-666666666666
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

See [CLI](../cli) and [Other](other) and [External](https://example.com) and [Anchor](#section).
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	result, err := fixer.Fix(path)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(result.LinkStyleUpdates) == 0 {
		t.Errorf("expected LinkStyleUpdates to be populated, got 0")
		return
	}
	if result.LinkStyleUpdates[0].Updated < 2 {
		t.Errorf("expected ≥2 links updated (cli and other), got %d", result.LinkStyleUpdates[0].Updated)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !containsBytes(out, "[CLI](../cli.md)") {
		t.Errorf("expected ../cli → ../cli.md rewrite, got:\n%s", got)
	}
	if !containsBytes(out, "[Other](other.md)") {
		t.Errorf("expected other → other.md rewrite, got:\n%s", got)
	}
	if !containsBytes(out, "[External](https://example.com)") {
		t.Errorf("external link should be left alone, got:\n%s", got)
	}
	if !containsBytes(out, "[Anchor](#section)") {
		t.Errorf("anchor should be left alone, got:\n%s", got)
	}
}

func TestFixer_InternalLinkStyleDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 77777777-aaaa-aaaa-aaaa-777777777777
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

See [CLI](../cli).
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, true, false).WithAutoConfirm(true)
	result, err := fixer.Fix(path)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(result.LinkStyleUpdates) == 0 {
		t.Errorf("expected dry-run to still report LinkStyleUpdates")
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if containsBytes(out, "../cli.md") {
		t.Errorf("dry-run should not modify file, got:\n%s", string(out))
	}
}
