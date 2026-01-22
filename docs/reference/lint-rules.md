---
aliases:
  - /_uid/cb491357-fc40-4fee-bddc-f68fee69c437/
categories:
  - reference
date: 2025-12-29T00:00:00Z
fingerprint: 463a9db13e57b5a8cf74a9e1202ea2d9b4dbc5c808ba1d11c57774a135af85dc
lastmod: "2026-01-22"
tags:
  - linting
  - validation
  - rules
title: Lint Rules Reference
uid: cb491357-fc40-4fee-bddc-f68fee69c437
---

# Lint Rules Reference

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

**Auto-fix**: Replaces special characters with hyphens: `file@name.md` → `file-name.md`

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

### Rule: Reserved Names

**Pattern**: Filenames that conflict with Hugo taxonomy URLs

**Reserved Names**: `tags.md`, `categories.md` (without namespace prefix)

**Rationale**:
- Hugo reserves `/tags/` and `/categories/` URLs for taxonomy listings
- Direct files cause URL conflicts and build errors

**Examples**:

```
❌ Invalid:
  tags.md
  categories.md

✅ Valid (with prefix):
  content-tags.md
  doc-categories.md
  using-tags.md
  about-categories.md
```

**Auto-fix**: Adds prefix: `tags.md` → `content-tags.md`

**Error Message**:
```
ERROR: Reserved filename
  File: docs/tags.md
  Issue: Conflicts with Hugo taxonomy URL
  
  Current:  docs/tags.md
  Suggested: docs/content-tags.md
  
  Why: Hugo reserves /tags/ for taxonomy listings.
  Using tags.md creates URL conflicts.
  
  Fix: docbuilder lint --fix docs/
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

**Auto-fix**: Not automatically fixable (can't determine intent)

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

Structure rules are **Warnings** that don't block builds but should be addressed.

### Rule: Missing Section Index

**Pattern**: Directory contains `.md` files but no `_index.md`

**Rationale**:
- Section won't appear in navigation sidebar
- Directory appears empty in site structure
- No landing page for the section

**Examples**:

```
❌ Missing _index.md:
docs/
  api/
    authentication.md
    authorization.md
    # No _index.md

✅ Has _index.md:
docs/
  api/
    _index.md          ← Section landing page
    authentication.md
    authorization.md
```

**Auto-fix**: Can generate basic `_index.md` with `--fix --generate-indexes`

**Warning Message**:
```
WARNING: Missing section index
  Directory: docs/api/
  Issue: Contains 5 markdown files but no _index.md
  Impact: Section will not appear in navigation sidebar
  
  Create _index.md to define this section:
  
  ---
  title: "API Documentation"
  weight: 2
  ---
  
  This section contains API guides and references.
```

---

### Rule: Deep Nesting

**Pattern**: Directory structure exceeds 4 levels deep

**Rationale**:
- Poor navigation UX (too many clicks)
- Overly complex information architecture
- Consider flattening or reorganizing

**Examples**:

```
❌ Too deep (5 levels):
docs/guides/advanced/api/rest/authentication.md

⚠️ Consider flattening:
docs/guides/api-rest-authentication.md
# or
docs/api/rest-authentication.md
```

**Auto-fix**: Not automatically fixable (requires architectural decision)

**Warning Message**:
```
WARNING: Deep directory nesting
  File: docs/guides/advanced/api/rest/authentication.md
  Depth: 5 levels
  
  Consider flattening directory structure for better UX.
  Deep nesting makes navigation difficult.
  
  Suggested: docs/api/rest-authentication.md
```

---

### Rule: Orphaned Assets

**Pattern**: Image file not referenced by any markdown file

**Rationale**:
- Bloats repository size
- May be leftover from deletions
- Indicates maintenance needed

**Examples**:

```
❌ Orphaned:
docs/
  images/
    old-screenshot.png  # Not referenced anywhere
```

**Auto-fix**: Can list orphans with `--fix --remove-orphans` (prompts for confirmation)

**Warning Message**:
```
WARNING: Orphaned asset
  File: docs/images/old-screenshot.png
  Issue: Not referenced by any markdown file
  Size: 2.4 MB
  
  This file may be unused and safe to remove.
  
  To remove: docbuilder lint --fix --remove-orphans
```

---

### Rule: Mixed Naming Styles

**Pattern**: Same directory has different naming conventions

**Rationale**:
- Inconsistent developer experience
- Harder to remember filenames
- Looks unprofessional

**Examples**:

```
❌ Mixed styles:
docs/
  getting_started.md    # snake_case
  api-guide.md          # kebab-case
  UserManual.md         # PascalCase
  CHANGELOG.md          # SCREAMING_CASE

✅ Consistent:
docs/
  getting-started.md
  api-guide.md
  user-manual.md
  changelog.md
```

**Auto-fix**: Normalizes to kebab-case: `getting_started.md` → `getting-started.md`

**Warning Message**:
```
WARNING: Mixed filename styles
  Directory: docs/
  Issue: Files use different naming conventions
  
  Files:
    • getting_started.md (snake_case)
    • api-guide.md (kebab-case)
    • UserManual.md (PascalCase)
  
  Recommended: Use consistent kebab-case naming
  
  Fix: docbuilder lint --fix docs/
```

---

## Asset Rules

Asset rules apply to images and other non-markdown files.

### Rule: Image Filename Issues

**Severity**: Warning

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

### Rule: Absolute URLs to Internal Assets

**Severity**: Warning

**Pattern**: `![](https://your-domain.com/images/logo.png)` pointing to same repository

**Rationale**:
- Breaks in local development
- Not portable across deployments
- Should use relative paths

**Examples**:

```
❌ Absolute (internal):
![Logo](https://docs.example.com/images/logo.png)

✅ Relative:
![Logo](./images/logo.png)
![Logo](../assets/logo.png)
```

**Auto-fix**: Not automatically fixable (requires manual verification)

**Warning Message**:
```
WARNING: Absolute URL to internal asset
  File: docs/index.md
  Line: 8
  Issue: Using absolute URL for same-repository asset
  
  ![Logo](https://docs.example.com/images/logo.png)
  
  This breaks in local development and isn't portable.
  
  Use relative path instead:
  ![Logo](./images/logo.png)
```

---

### Rule: Large Binary Files

**Severity**: Warning

**Pattern**: Image/asset file exceeds 5 MB

**Rationale**:
- Slows Git operations (clone, fetch, pull)
- Poor repository performance
- Consider external hosting or optimization

**Examples**:

```
⚠️ Large files:
  screenshot-4k.png (12 MB)
  demo-video.gif (25 MB)
  presentation.pdf (18 MB)
```

**Auto-fix**: Not automatically fixable (requires manual optimization)

**Warning Message**:
```
WARNING: Large binary file
  File: docs/images/screenshot-4k.png
  Size: 12.4 MB
  
  Large files slow Git performance and bloat repository.
  
  Consider:
    • Optimize/compress image (use tools like tinypng.com)
    • Host externally (CDN, cloud storage)
    • Split into multiple smaller images
    • Use thumbnail with link to full size
```

---

## Standard File Exclusions

These files are **automatically excluded** from linting (never reported as errors):

- `README.md` (root or in subdirectories)
- `CONTRIBUTING.md`
- `CHANGELOG.md`
- `LICENSE.md`
- `CODE_OF_CONDUCT.md`
- Files in `.git/` directory
- Files matching `.gitignore` patterns

**Rationale**: These are standard repository files, not documentation content processed by Hugo.

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
| Uppercase letters | ✅ Yes | Convert to lowercase |
| Spaces | ✅ Yes | Replace with hyphens |
| Special characters | ✅ Yes | Replace with hyphens |
| Leading/trailing hyphens | ✅ Yes | Strip |
| Double extensions | ❌ No | Manual removal |
| Reserved names | ✅ Yes | Add prefix |
| Malformed frontmatter | ❌ No | Manual correction |
| Broken links | ❌ No* | Manual fix (*Can detect only) |
| Missing section index | ⚠️ Partial | Generate basic `_index.md` |
| Mixed naming styles | ✅ Yes | Normalize to kebab-case |
| Image filename issues | ✅ Yes | Same as markdown files |
| Large binary files | ❌ No | Manual optimization |

---

## See Also

- [Setup Linting](../how-to/setup-linting.md) - Installation and usage guide
- [CI/CD Integration](../how-to/ci-cd-linting.md) - Automated validation
- [Migration Guide](../how-to/migrate-to-linting.md) - Cleaning up existing repositories
- [ADR-005: Documentation Linting](../adr/adr-005-documentation-linting.md) - Architecture decision
