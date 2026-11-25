---
aliases:
  - /_uid/cb491357-fc40-4fee-bddc-f68fee69c437/
categories:
  - reference
date: 2025-12-29T00:00:00Z
fingerprint: 7019828e0afd6d49aa465bf9509b7bf12611349cb31e15752bc61b2d1cb7a18a
lastmod: "2026-11-25"
tags:
  - linting
  - validation
  - rules
title: Lint Rules Reference
uid: cb491357-fc40-4fee-bddc-f68fee69c437
---

Complete reference for all documentation linting rules enforced by DocBuilder.

## Overview

DocBuilder's linter enforces **opinionated, non-configurable** rules based on Hugo and DocBuilder best practices. Rules are classified by severity:

| Severity | Impact | Build Behavior |
|----------|--------|----------------|
| **Error** | Blocks build | Exit code 2, CI fails |
| **Warning** | Should fix | Exit code 1, CI passes |
| **Info** | Informational | Exit code 0, CI passes |

## Rule Categories

- [Filename Rules](#filename-rules) (Errors)
- [Content Rules](#content-rules) (Errors)
- [Structure Rules](#structure-rules) (Warnings)
- [Asset Rules](#asset-rules) (Warnings/Info)

---

## Filename Rules

All filename rules are **Errors** that block the build.

### Rule: Uppercase Letters

**Pattern**: `[A-Z]` in filename

**Rationale**: 
- Filenames become URL slugs in Hugo
- Uppercase causes case-sensitivity issues across platforms (macOS/Windows case-insensitive, Linux case-sensitive)
- Creates inconsistent URLs: `API-Guide.md` → `/API-Guide/` vs expected `/api-guide/`

**Examples**:

```
❌ Invalid:
  API-Guide.md
  MyDocument.md
  UserManual.md
  README_FIRST.md

✅ Valid:
  api-guide.md
  my-document.md
  user-manual.md
  readme-first.md
```

**Auto-fix**: Converts to lowercase: `API-Guide.md` → `api-guide.md`

**Error Message**:
```
ERROR: Invalid filename
  File: docs/API-Guide.md
  Issue: Contains uppercase letters (A, P, I, G)
  
  Current:  docs/API-Guide.md
  Suggested: docs/api-guide.md
  
  Why: Uppercase letters cause case-sensitivity issues 
  across platforms and create inconsistent URLs.
  
  Fix: docbuilder lint --fix docs/
```

---

### Rule: Spaces in Filename

**Pattern**: `[ ]` (space character) in filename

**Rationale**:
- Spaces become `%20` in URLs: `My Doc.md` → `/my%20doc/`
- Breaks some link parsers and shell commands
- Poor user experience with encoded URLs

**Examples**:

```
❌ Invalid:
  My Document.md
  API Guide.md
  User Manual v2.md
  Architecture Diagram.png

✅ Valid:
  my-document.md
  api-guide.md
  user-manual-v2.md
  architecture-diagram.png
```

**Auto-fix**: Replaces spaces with hyphens: `My Document.md` → `my-document.md`

**Error Message**:
```
ERROR: Invalid filename
  File: docs/My Document.md
  Issue: Contains space characters
  
  Current:  docs/My Document.md
  Suggested: docs/my-document.md
  
  Why: Spaces create problematic URLs (/my%20document/) 
  and may break cross-references.
  
  Fix: docbuilder lint --fix docs/
```

---

### Rule: Special Characters

**Pattern**: Any character not in `[a-z0-9-_.]`

**Rationale**:
- Special characters unsupported by Hugo's URL generation
- May require shell escaping in commands
- Can break on different filesystems

**Invalid Characters**: `@`, `#`, `$`, `%`, `&`, `*`, `(`, `)`, `[`, `]`, `{`, `}`, `;`, `:`, `'`, `"`, `<`, `>`, `?`, `|`, `\`, `/`, `~`, `` ` ``

**Examples**:

```
❌ Invalid:
  file@name.md
  doc#tag.md
  guide(final).md
  config[prod].md
  notes&ideas.md

✅ Valid:
  file-name.md
  doc-tag.md
  guide-final.md
  config-prod.md
  notes-ideas.md
```

**Auto-fix**: Removes the special characters: `file@name.md` → `filename.md`

**Error Message**:
```
ERROR: Invalid filename
  File: docs/config[prod].md
  Issue: Contains special characters: [ ]
  
  Current:  docs/config[prod].md
  Suggested: docs/config-prod.md
  
  Why: Special characters are unsupported by Hugo and 
  may cause issues on different filesystems.
  
  Fix: docbuilder lint --fix docs/
```

---

### Rule: Leading/Trailing Hyphens or Underscores

**Pattern**: Filename starts or ends with `-` or `_`

**Rationale**:
- Creates malformed URLs: `/-docs/` or `/_temp/`
- May be interpreted as hidden files on Unix systems
- Poor aesthetics and confusing navigation

**Examples**:

```
❌ Invalid:
  -draft.md
  _temporary.md
  config-.md
  notes_.md

✅ Valid:
  draft.md
  temporary.md
  config.md
  notes.md
```

**Auto-fix**: Strips leading/trailing hyphens and underscores: `-draft.md` → `draft.md`

**Error Message**:
```
ERROR: Invalid filename
  File: docs/-draft.md
  Issue: Leading hyphen
  
  Current:  docs/-draft.md
  Suggested: docs/draft.md
  
  Why: Leading/trailing hyphens create malformed URLs 
  and may be interpreted as hidden files.
  
  Fix: docbuilder lint --fix docs/
```

---

### Rule: Invalid Double Extensions

**Pattern**: Multiple file extensions (e.g., `.md.backup`, `.markdown.old`)

**Rationale**:
- Hugo attempts to process files with `.md` or `.markdown` anywhere in the extension
- Backup files, temp files cause build errors
- Whitelisted exceptions exist for embedded diagram formats

**Whitelisted Double Extensions**:
- `.drawio.png` - Draw.io embedded PNG diagrams (editable)
- `.drawio.svg` - Draw.io embedded SVG diagrams (editable)

**Examples**:

```
❌ Invalid:
  api-guide.md.backup
  notes.markdown.old
  config.md.txt
  draft.md.2024-12-29

✅ Valid (single extension):
  api-guide.md
  notes.markdown
  config.txt
  draft.md

✅ Valid (whitelisted):
  architecture.drawio.png
  flowchart.drawio.svg
```

**Auto-fix**: Not automatically fixable (requires manual intervention)

**Error Message**:
```
ERROR: Invalid double extension
  File: docs/api-guide.md.backup
  Issue: Contains non-whitelisted double extension .md.backup
  
  Current:  docs/api-guide.md.backup
  
  Hugo will attempt to process this as markdown, 
  causing build errors.
  
  Whitelisted double extensions (allowed):
    • .drawio.png (Draw.io embedded PNG diagrams)
    • .drawio.svg (Draw.io embedded SVG diagrams)
  
  How to fix:
    1. Remove backup files from docs directory
    2. Use Git history or separate backup location
    3. Add to .gitignore: *.backup
```

---

## Content Rules

Content rules validate the internal structure of markdown files.

### Rule: Malformed Frontmatter YAML

**Pattern**: Invalid YAML syntax in frontmatter block

**Rationale**:
- Hugo silently skips pages with invalid frontmatter
- Page won't render but no error is shown
- Causes missing documentation without obvious cause

**Examples**:

```yaml
❌ Invalid:
---
title: API Guide
date: 2025-12-29
invalid key without colon
---

❌ Invalid (indentation):
---
title: API Guide
  date: 2025-12-29
---

❌ Invalid (missing closing):
---
title: API Guide
date: 2025-12-29

# Content starts here
```

```yaml
✅ Valid:
---
title: "API Guide"
date: 2025-12-29
tags:
  - api
  - reference
---
```

**Auto-fix**: Not automatically fixable (requires manual correction)

**Error Message**:
```
ERROR: Invalid frontmatter
  File: docs/api-guide.md
  Line: 4
  Issue: YAML parsing failed: mapping values are not allowed here
  
  ---
  title: API Guide
  date: 2025-12-29
  invalid key without colon
  ---
  
  Frontmatter must be valid YAML enclosed by --- markers.
  Check for proper indentation and key: value format.
```

---

### Rule: Missing Frontmatter Closing

**Pattern**: Frontmatter starts with `---` but missing closing `---`

**Rationale**:
- Hugo treats entire file as frontmatter
- No content rendered
- Page appears blank

**Examples**:

```markdown
❌ Invalid:
---
title: API Guide
date: 2025-12-29

# API Guide

This is the content...
```

```markdown
✅ Valid:
---
title: API Guide
date: 2025-12-29
---

# API Guide

This is the content...
```

**Auto-fix**: Not automatically fixable (ambiguous where frontmatter should end)

**Error Message**:
```
ERROR: Incomplete frontmatter
  File: docs/api-guide.md
  Issue: Missing closing --- marker
  
  Frontmatter started at line 1 but never closed.
  Hugo will treat the entire file as frontmatter,
  resulting in a blank page.
  
  Add --- after frontmatter block before content.
```

---

### Rule: Frontmatter Fingerprint

**Pattern**: Missing `fingerprint:` field, or fingerprint does not match content

**Rationale**:
- DocBuilder uses a frontmatter fingerprint to detect content changes reliably
- Helps avoid stale pages where content changed but metadata did not
- Enables deterministic updates of `lastmod` only when content meaningfully changes

**Examples**:

```yaml
❌ Invalid (missing fingerprint):
---
title: "API Guide"
date: 2025-12-29
---

❌ Invalid (stale fingerprint):
---
title: "API Guide"
date: 2025-12-29
fingerprint: deadbeef
---

✅ Valid:
---
title: "API Guide"
date: 2025-12-29
fingerprint: <generated>
---
```

**Auto-fix**:
- Regenerates `fingerprint` via `docbuilder lint --fix`
- If the fingerprint value changes, sets/updates `lastmod` to today’s UTC date (`YYYY-MM-DD`)

**Error Message**:
```
ERROR: Missing or invalid fingerprint in frontmatter
  File: docs/api-guide.md
  Issue: fingerprint is missing or does not match the document content

  Fix: docbuilder lint --fix docs/
```

---

### Rule: Broken Internal Links

**Pattern**: Link to non-existent local file

**Rationale**:
- Broken links lead to 404 errors in production
- Poor user experience
- Indicates stale or incorrect documentation

**Examples**:

```markdown
❌ Invalid:
[API Guide](./api-guide.md)  # File doesn't exist
[Configuration](../config/settings.md)  # File doesn't exist
![Diagram](./images/flow.png)  # Image doesn't exist
```

```markdown
✅ Valid:
[API Guide](./getting-started.md)  # File exists
[Configuration](../config/readme.md)  # File exists
![Diagram](./images/architecture.png)  # Image exists
```

**Auto-fix**: Healed when the file was renamed in the repository: the
link is rewritten to the new name. The rename has to be found in the
history of git, and it has to be unambiguous (ADR-012). A link to a file
that never existed is not fixable.

**Error Message**:
```
ERROR: Broken internal link
  File: docs/index.md
  Line: 15
  Issue: Link target does not exist
  
  [API Guide](./api-guide.md)
              ^^^^^^^^^^^^^^^^^
  
  Target file not found: docs/api-guide.md
  
  Possible fixes:
    • Create the missing file
    • Update link to correct path
    • Remove broken link
```

---

## Structure Rules

The rule of the structure of the documentation set is the
**directory-category-consistency**: a document under a mode-specific
directory carries the category of that directory.

## Asset Rules

Asset rules apply to images and other non-markdown files.

### Rule: Image Filename Issues

**Severity**: Error

**Pattern**: Same filename rules as markdown (spaces, uppercase, special chars)

**Rationale**:
- Same URL and filesystem concerns as markdown
- Image paths must be exact matches (case-sensitive)

**Examples**:

```
❌ Warning:
  Screenshot 2024.png
  Company_Logo.svg
  Diagram(final).png

✅ Valid:
  screenshot-2024.png
  company-logo.svg
  diagram-final.png
```

**Auto-fix**: Applies same transformations as markdown files

**Warning Message**:
```
WARNING: Invalid image filename
  File: docs/images/Screenshot 2024.png
  Issue: Contains spaces and uppercase letters
  
  Current:  docs/images/Screenshot 2024.png
  Suggested: docs/images/screenshot-2024.png
  
  Asset files follow the same rules as markdown files.
  Image references in markdown will be updated automatically.
  
  Fix: docbuilder lint --fix docs/
```

---

### Rule: Whitelisted Double Extensions

**Severity**: Info (explicitly allowed)

**Pattern**: `.drawio.png` or `.drawio.svg` files

**Rationale**:
- Draw.io exports editable diagrams as embedded format
- PNG/SVG contains both image and diagram source data
- Allows round-trip editing without separate source files

**Examples**:

```
✅ Explicitly allowed:
  architecture.drawio.png
  flowchart.drawio.svg
  system-design.drawio.png
```

**Info Message**:
```
INFO: Whitelisted double extension
  File: docs/diagrams/architecture.drawio.png
  
  This double extension is explicitly whitelisted for 
  Draw.io embedded diagrams (editable format).
```

---

## Standard File Exclusions

These files are **automatically excluded** from linting (never reported as errors):

- `README.md` (at the root of the linted tree)
- `CONTRIBUTING.md` (at the root of the linted tree)
- `CHANGELOG.md` (at the root of the linted tree)
- `LICENSE.md` (at the root of the linted tree)
- `CODE_OF_CONDUCT.md` (at the root of the linted tree)
- Files in `.git/` directory

**Rationale**: These are standard repository files, not documentation content processed by Hugo.

The name of a conventional file is never renamed, in any directory: the
git forges recognize `README.md` and its kin, and a `readme.md` is no
home page.

---

## Rule Evolution

Linting rules evolve with DocBuilder versions. See [Lint Rules Changelog](./lint-rules-changelog.md) for version history.

| DocBuilder Version | Rule Changes |
|--------------------|--------------|
| 1.0 - 1.5 | Initial filename and frontmatter rules |
| 1.6+ | Enhanced frontmatter schema validation (future) |
| 2.0+ | Asset transformation rules (future) |

---

## Exit Codes Summary

| Code | Meaning | Condition |
|------|---------|-----------|
| `0` | Success | No issues found (clean) |
| `1` | Warnings | Warnings present, no errors |
| `2` | Errors | Errors found (blocks build) |
| `3` | Failure | Linter execution error |

---

## Auto-Fix Capabilities

| Rule | Auto-Fixable | Method |
|------|--------------|--------|
| `filename-conventions` | ✅ Yes | The name is written in lower case, with hyphens |
| `sequence-prefix-filename` | ✅ Yes | The file is renamed with its number |
| `frontmatter-uid` | ✅ Yes | The uid and its alias are written |
| `frontmatter-fingerprint` | ✅ Yes | The fingerprint is written |
| `frontmatter-required-fields` | ✅ Yes | The missing fields are written |
| `body-h1` | ❌ No | Manual correction |
| `tag-count` | ❌ No | Manual correction |
| `category-naming` | ✅ Yes | The category is renamed |
| `cross-mode-category` | ❌ No | Manual correction |
| `directory-category-consistency` | ✅ Yes | The category of the directory is written |
| `template-uid-binding` | ✅ Yes | The uid is bound in the template |
| `broken-links` | ✅ Yes* | The link is rewritten when the file was renamed in git (*ADR-012) |

---

## See Also

- [Setup Linting](../how-to/setup-linting.md) - Installation and usage guide
- [CI/CD Integration](../how-to/ci-cd-linting.md) - Automated validation
- [Migration Guide](../how-to/migrate-to-linting.md) - Cleaning up existing repositories
- [ADR-005: Documentation Linting](../adr/adr-005-documentation-linting.md) - Architecture decision
