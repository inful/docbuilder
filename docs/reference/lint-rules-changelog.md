---
categories:
    - reference
date: 2025-12-29T00:00:00Z
id: 03dda7dd-fb12-464d-b66e-f69044cb83d3
tags:
    - linting
    - changelog
    - versions
title: Lint Rules Changelog
---

# Lint Rules Changelog

This document tracks changes to linting rules across DocBuilder versions, helping teams understand when rules were added, changed, or deprecated.

## Version History

### v1.0.0 (2025-12-29) - Initial Release

**Initial linting implementation with core validation rules.**

#### Filename Rules (Errors)

**Added:**
- ✅ Uppercase letter detection
- ✅ Space character detection
- ✅ Special character detection (not `[a-z0-9-_.]`)
- ✅ Leading/trailing hyphen and underscore detection
- ✅ Invalid double extension detection
- ✅ Reserved filename detection (`tags.md`, `categories.md`)

**Whitelisted Extensions:**
- ✅ `.drawio.png` - Draw.io embedded PNG diagrams
- ✅ `.drawio.svg` - Draw.io embedded SVG diagrams

**Standard File Exclusions:**
- ✅ `README.md`
- ✅ `CONTRIBUTING.md`
- ✅ `CHANGELOG.md`
- ✅ `LICENSE.md`
- ✅ `CODE_OF_CONDUCT.md`

#### Auto-Fix Capabilities

**Added:**
- ✅ File renaming with `git mv` support
- ✅ Lowercase conversion
- ✅ Space to hyphen replacement
- ✅ Special character to hyphen replacement
- ✅ Leading/trailing character stripping
- ✅ Internal link resolution and updates
- ✅ Anchor fragment preservation in links
- ✅ Reference-style link updates
- ✅ Image link updates
- ✅ Dry-run mode (`--fix --dry-run`)
- ✅ Interactive confirmation prompts
- ✅ Detailed fix reports

#### Link Resolution

**Added:**
- ✅ Relative path resolution (e.g., `./file.md`, `../dir/file.md`)
- ✅ Hugo site-absolute path support (e.g., `/docs/file`)
- ✅ Inline link detection and updates
- ✅ Reference-style link detection and updates
- ✅ Image link detection and updates
- ✅ Anchor fragment preservation (`#section`)
- ✅ External URL exclusion (don't modify `https://...`)
- ✅ Code block exclusion (don't modify links in code)
- ✅ Broken link detection and reporting

#### CLI Features

**Added:**
- ✅ `docbuilder lint [path]` command
- ✅ Intelligent path detection (`docs/`, `documentation/`, fallback to `.`)
- ✅ Exit codes: 0 (clean), 1 (warnings), 2 (errors), 3 (failure)
- ✅ Output formats: `--format=text|json`
- ✅ Verbosity: `--quiet`, `--verbose`
- ✅ Color support with `NO_COLOR` detection
- ✅ `--fix` flag for auto-fixing
- ✅ `--fix --dry-run` for preview
- ✅ `--fix --yes` for non-interactive mode

#### Git Hooks

**Added:**
- ✅ Traditional pre-commit hook script
- ✅ `docbuilder lint install-hook` command
- ✅ Lefthook configuration example
- ✅ Staged file filtering

#### CI/CD Integration

**Added:**
- ✅ GitHub Actions workflow example
- ✅ GitLab CI template
- ✅ JSON output schema
- ✅ PR comment integration examples

#### Testing

**Added:**
- ✅ Golden test framework for lint validation
- ✅ Auto-fix integration tests
- ✅ Link resolution test suite
- ✅ Lint-DocBuilder sync tests
- ✅ Rule drift detection CI workflow

#### Bug Fixes

**Fixed:**
- ✅ Link transformation preserves `./` prefix (Issue: `./file.md` became `/repo/./file`)
- ✅ Linter now resolves Hugo site-absolute paths (`/docs/file`)
- ✅ Linter tries `.md` and `.markdown` extensions when validating links

---

## Future Planned Changes

### v1.1.0 (Planned)

**Content Rules (Errors) - In Development:**
- [ ] Malformed frontmatter YAML detection
- [ ] Missing frontmatter closing marker detection
- [ ] Invalid frontmatter key detection (duplicates)
- [ ] Broken internal link detection (enhanced)
- [ ] Image reference validation

**Structure Rules (Warnings) - Planned:**
- [ ] Missing section index detection (`_index.md`)
- [ ] Deep directory nesting detection (>4 levels)
- [ ] Orphaned asset detection (unreferenced images)
- [ ] Mixed naming style detection in same directory

**Asset Rules (Warnings/Info) - Planned:**
- [ ] Image filename validation (same as markdown)
- [ ] Absolute URL to internal asset detection
- [ ] Large binary file detection (>5 MB)

### v1.2.0+ (Future)

**Advanced Features - Roadmap:**
- [ ] Frontmatter schema validation (structured fields)
- [ ] Content linting (spell checking, grammar)
- [ ] Markdown style consistency (headings, lists)
- [ ] Accessibility checks (alt text, heading hierarchy)
- [ ] SEO recommendations (meta descriptions)
- [ ] VS Code extension integration
- [ ] Language Server Protocol (LSP) support

---

## Migration Guide

### Upgrading to v1.0.0

**Initial adoption for existing repositories:**

1. **Run discovery**:
   ```bash
   docbuilder lint --format=json > lint-issues.json
   ```

2. **Review issues**:
   ```bash
   docbuilder lint
   ```

3. **Preview fixes**:
   ```bash
   docbuilder lint --fix --dry-run
   ```

4. **Apply fixes**:
   ```bash
   docbuilder lint --fix
   ```

5. **Commit cleanup**:
   ```bash
   git add -A
   git commit -m "docs: normalize filenames for linting compliance"
   ```

**Breaking Changes**: None (initial release)

**Deprecations**: None (initial release)

---

## Rule Severity Changes

No severity changes in v1.0.0 (initial release).

**Future considerations:**
- Warnings may be promoted to errors based on usage data
- New rules start as warnings before becoming errors
- Breaking changes announced 1 version ahead

---

## Compatibility Matrix

| DocBuilder Version | Linter Version | Go Version | Hugo Version |
|-------------------|----------------|------------|--------------|
| 1.0.0 | 1.0.0 | 1.21+ | 0.112+ |

---

## Rule Statistics

### v1.0.0 Coverage

| Category | Rules | Errors | Warnings | Info |
|----------|-------|--------|----------|------|
| Filename | 6 | 6 | 0 | 0 |
| Content | 0 | 0 | 0 | 0 |
| Structure | 0 | 0 | 0 | 0 |
| Assets | 1 | 0 | 0 | 1 |
| **Total** | **7** | **6** | **0** | **1** |

**Auto-fixable**: 5 / 6 error rules (83%)

---

## Feedback and Evolution

### How Rules Are Added

1. **Proposal**: Issue or discussion in DocBuilder repository
2. **Review**: Core team evaluates against best practices
3. **Implementation**: Add to linter with tests
4. **Documentation**: Update this changelog and rule reference
5. **Release**: Included in next version with migration notes

### Reporting Issues

If you encounter:
- **False positives**: Rule incorrectly flags valid content
- **False negatives**: Rule misses actual issues
- **Unclear messages**: Error/warning text confusing

Please file an issue with:
- Linter version: `docbuilder --version`
- Minimal reproduction case
- Expected vs actual behavior
- Suggested improvement

### Requesting New Rules

When requesting new rules, include:
- **Use case**: What problem does this solve?
- **Examples**: Show invalid patterns to detect
- **Rationale**: Why is this a best practice?
- **Severity**: Should it be error, warning, or info?
- **Auto-fix**: Can it be automatically fixed?

---

## Version Compatibility

### Semantic Versioning

Linter rules follow semantic versioning:

- **Major** (x.0.0): Breaking changes, rule removals
- **Minor** (1.x.0): New rules, new features
- **Patch** (1.0.x): Bug fixes, message improvements

### Backward Compatibility

- **Error rules**: Never removed without deprecation period
- **Warning rules**: May be promoted to errors with notice
- **Auto-fix behavior**: Changes announced in advance
- **Exit codes**: Stable contract across versions

---

## Sync with DocBuilder

Linting rules stay synchronized with DocBuilder behavior:

| DocBuilder Change | Linter Update |
|-------------------|---------------|
| New Hugo feature support | Add validation rules |
| Deprecate functionality | Add deprecation warnings |
| Bug fix in processing | Update corresponding rule |
| New file type support | Extend validation |

**Sync verification**: Weekly CI job tests linter against actual DocBuilder builds.

---

## See Also

- [Lint Rules Reference](./lint-rules.md) - Complete rule documentation
- [Setup Linting](../how-to/setup-linting.md) - Installation and usage
- [Migration Guide](../how-to/migrate-to-linting.md) - Adopt linting in existing repos
- [ADR-005: Documentation Linting](../adr/adr-005-documentation-linting.md) - Design decisions

---

**Last Updated**: 2025-12-29  
**Linter Version**: 1.0.0  
**DocBuilder Version**: 1.0.0
