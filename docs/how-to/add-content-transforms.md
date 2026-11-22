---
aliases:
  - /_uid/8f736168-8777-470e-a8a4-ddb9209a073b/
categories:
  - how-to
date: 2025-12-15T00:00:00Z
fingerprint: ef911e788f65bb12e4b313e6c6137081dc9cb5e2aa1905e2861704ec0ac4dfa8
lastmod: "2026-11-22"
tags:
  - content-transforms
  - development
  - customization
title: 'How To: Add Content Transforms'
uid: 8f736168-8777-470e-a8a4-ddb9209a073b
---

This guide shows you how to add a custom transformation to the markdown
files before Hugo renders them.

## Overview

DocBuilder processes every discovered markdown file through a fixed
pipeline of transforms. A transform is a plain function that receives a
`Document` and mutates it in place.

The pipeline lives in `internal/hugo/pipeline/`. The transforms are
listed, in order, in `defaultTransforms` in `processor.go`:

```go
func defaultTransforms(cfg *config.Config) []FileTransform {
	return []FileTransform{
		parseFrontMatter,                  // 1. Parse YAML front matter from content
		normalizeIndexFiles,               // 2. Rename README to _index
		buildBaseFrontMatter,              // 3. Build base front matter structure
		// ...
		serializeDocument,                 // 14. Serialize to final bytes (FM + content)
		fingerprintContent,                // 15. Add content fingerprint (must be last)
	}
}
```

There is no registry and no priority: the slice order *is* the execution
order. See [ADR-003](../adr/adr-003-fixed-transform-pipeline.md) for why
the registry was removed.

## The Transform Signature

```go
type FileTransform func(doc *Document) ([]*Document, error)
```

A transform:

- mutates `doc` in place (`doc.Content`, `doc.FrontMatter`);
- returns `nil, nil` for the common case;
- returns extra documents to inject them into the pipeline. A transform
  that returns documents may not run on a generated document: the
  pipeline rejects that, to avoid an infinite loop.

`Document` (in `document.go`) carries the markdown body, the front
matter map, and the metadata the discovery phase collected: the Hugo
content path, the repository, the section, the source commit and URL.

## Adding a Transform

### Step 1: Write the transform

Create a file in `internal/hugo/pipeline/`, one transform per file:

```go
package pipeline

import "strings"

// escapeGitHubAlerts rewrites GitHub-style alert markers as Hugo
// shortcodes, so the theme renders them as callouts.
func escapeGitHubAlerts(doc *Document) ([]*Document, error) {
	doc.Content = strings.ReplaceAll(doc.Content, ":warning:", `{{</* shortcode */>}}`)
	return nil, nil
}
```

### Step 2: Register it in the pipeline order

Add the function to the `defaultTransforms` slice at the position where
it belongs:

```go
		escapeShortcodesInCodeBlocks,      // 7. Escape Hugo shortcodes in code blocks
		escapeGitHubAlerts,                // 8. Rewrite GitHub alert markers
		rewriteRelativeLinks(cfg),         // 9. Fix markdown links
```

Order matters. A transform that reads the front matter must run after
`buildBaseFrontMatter`; a transform that rewrites the content must run
before `serializeDocument`, which freezes the content into `doc.Raw`.
`fingerprintContent` must stay last: it hashes the serialized output.

### Step 3: Test it

```go
func TestEscapeGitHubAlerts(t *testing.T) {
	doc := &Document{Content: "note: :warning: careful"}
	if _, err := escapeGitHubAlerts(doc); err != nil {
		t.Fatalf("escapeGitHubAlerts: %v", err)
	}
	if !strings.Contains(doc.Content, "alert") {
		t.Errorf("expected the alert marker to be rewritten, got %q", doc.Content)
	}
}
```

Run the pipeline tests with:

```bash
go test ./internal/hugo/pipeline/
```

## Common Use Cases

### Replace Text Across All Files

```go
func replaceDeprecatedSyntax(doc *Document) ([]*Document, error) {
	doc.Content = strings.ReplaceAll(doc.Content, ":information_source:", "ℹ️")
	return nil, nil
}
```

### Add a Front Matter Field

Front matter is a plain map, so a field is a map write:

```go
func addOwner(doc *Document) ([]*Document, error) {
	doc.FrontMatter["owner"] = doc.Repository
	return nil, nil
}
```

Write to `doc.FrontMatter`, not to `doc.OriginalFrontMatter`: the latter
holds the source front matter for reference and is not serialized.

### Generate a New Document

```go
func generateGlossary(doc *Document) ([]*Document, error) {
	if doc.Generated {
		return nil, nil // a generated document may not generate more documents
	}
	// ... build the new Document and return it
	return []*Document{newDoc}, nil
}
```

The new document is queued and runs through the whole pipeline from the
start.

## Best Practices

1. **Mutate in place.** The pipeline passes the same `*Document` down
   the chain; returning a modified copy instead of mutating loses the
   change.

2. **Do not touch `doc.Raw`.** It is set by `serializeDocument` and
   overwritten by `fingerprintContent`.

3. **Skip what does not apply.** Most transforms should no-op for the
   documents they do not handle (`if !doc.IsIndex { return nil, nil }`).

4. **Do not fail the build for cosmetic issues.** A fingerprint failure
   is logged, not fatal.

## Debugging Transforms

Build with verbose logging to see the pipeline run:

```bash
docbuilder build -c config.yaml -v
```

Inspect the output in the Hugo site:

```bash
cat site/content/repo/path/file.md
```

## See Also

- [ADR-003: Fixed Transform Pipeline](../adr/adr-003-fixed-transform-pipeline.md)
- [Architecture: Content Transform Pipeline](../explanation/comprehensive-architecture.md#content-transform-pipeline)
- [Architecture Overview](../explanation/architecture.md)
