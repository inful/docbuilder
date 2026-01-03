---
title: "ADR-005: Documentation Linting for Pre-Commit Validation"
date: 2025-12-29
categories:
  - architecture-decisions
tags:
  - linting
  - validation
  - documentation
  - developer-experience
weight: 5
---

# ADR-005: Documentation Linting for Pre-Commit Validation

Date: 2025-12-29

## Status

Proposed

## Context

DocBuilder processes documentation from multiple Git repositories, transforming markdown files into Hugo-compatible sites. Currently, developers discover issues only after committing and running builds:

### Current Pain Points

1. **Late feedback**: Filename issues (spaces, mixed-case) discovered during Hugo build
2. **Silent failures**: Invalid frontmatter causes pages to render incorrectly or not at all
3. **Broken links**: Cross-references break when files are renamed without updating links
4. **Path inconsistencies**: Mixed naming conventions (`README.md`, `api-guide.md`, `My Document.md`) in same repository
5. **Asset orphans**: Images referenced but not committed, or committed but never referenced
6. **Hugo quirks**: Reserved filenames (`_index.md` vs `index.md`) behave differently but look similar

### Impact

- Developers commit documentation that fails to build
- CI/CD pipelines fail unexpectedly
- Manual inspection required to diagnose issues
- Inconsistent documentation quality across repositories
- Time wasted on avoidable build failures

### Hugo and DocBuilder Best Practices

Hugo has specific expectations:
- Filenames become URL slugs: `My Document.md` → `/my%20document/` (problematic)
- Case sensitivity varies by OS: `README.md` vs `readme.md` causes issues
- Special files: `_index.md` (section landing), `index.md` (leaf bundle)
- Asset paths must match exactly (case-sensitive even on macOS/Windows during deployment)

DocBuilder's discovery system (`internal/docs/`) walks repositories and expects:
- Lowercase filenames for predictable Hugo paths
- No spaces or special characters (except `-`, `_`, `.`)
- Valid UTF-8 frontmatter
- Relative paths for cross-document links

## Decision

Implement a **documentation linting system** with multiple integration points:

1. **CLI command**: `docbuilder lint [path]` for manual validation
2. **Git hooks**: Traditional pre-commit hooks or lefthook for automatic checking
3. **CI/CD integration**: GitHub Actions / GitLab CI step for PR validation

### Architecture

```
internal/
  lint/
    linter.go              # Core linting engine
    rules.go               # Rule definitions and severity
    formatters.go          # Human-readable output
    fixer.go               # Automatic fix transformations
    
cmd/docbuilder/
  commands/
    lint.go                # CLI command implementation
    
scripts/
  install-hooks.sh         # Traditional pre-commit hook installer
  
lefthook.yml               # Lefthook configuration (optional)
  
.github/
  workflows/
    lint-docs.yml          # CI validation workflow
```

### Linting Rules

Rules are **fixed and opinionated** based on Hugo/DocBuilder best practices. No configuration override.

#### Filename Rules (Errors - Block Build)

**Allowed pattern**: `[a-z0-9-_.]` with exceptions for whitelisted double extensions:
- `.drawio.png` - Draw.io embedded PNG diagrams
- `.drawio.svg` - Draw.io embedded SVG diagrams

| Rule | Severity | Rationale |
|------|----------|-----------|
| Uppercase letters in filename | Error | Causes URL inconsistency, case-sensitivity issues |
| Spaces in filename | Error | Breaks Hugo URL generation, creates `%20` in paths |
| Special characters (not `[a-z0-9-_.]`) | Error | Unsupported by Hugo slugify, potential shell escaping issues |
| Leading/trailing hyphens or underscores | Error | Creates malformed URLs (`/-docs/` or `/_temp/`) |
| Double extensions (except whitelisted) | Error | Processed as markdown, causes build errors. Allowed: `.drawio.png`, `.drawio.svg` (embedded diagrams) |
| Reserved names without prefix (`tags.md`, `categories.md`) | Error | Conflicts with Hugo taxonomy URLs |

**Error Message Example:**
```
ERROR: Invalid filename detected
  File: docs/API Guide.md
  Issue: Contains space characters and uppercase letters
  Fix: Rename to: docs/api-guide.md
  
  Spaces in filenames create problematic URLs (/api%20guide/) 
  and may break cross-references. Hugo expects lowercase, 
  hyphen-separated filenames.
  
  To fix automatically: docbuilder lint --fix docs/
```

#### Content Rules (Errors - Block Build)

| Rule | Severity | Rationale |
|------|----------|-----------|
| Malformed frontmatter YAML | Error | Hugo fails to parse, page skipped silently |
| Missing closing `---` in frontmatter | Error | Entire file treated as frontmatter |
| Invalid frontmatter keys (duplicates) | Error | Undefined Hugo behavior |
| Broken internal links (`[text](./missing.md)`) | Error | 404s in production, poor UX |
| Image references to non-existent files | Error | Missing images break layout |

**Error Message Example:**
```
ERROR: Invalid frontmatter detected
  File: docs/installation.md
  Line: 3
  Issue: YAML parsing failed: mapping values are not allowed here
  
  ---
  title: Installation Guide
  date: 2025-12-29
  invalid key without colon
  ---
  
  Frontmatter must be valid YAML enclosed by --- markers.
  Check for proper indentation and key: value format.
```

#### Structure Rules (Warnings - Allow but Notify)

| Rule | Severity | Rationale |
|------|----------|-----------|
| Missing `_index.md` in directory with docs | Warning | Directory won't have landing page, appears empty in nav |
| Deeply nested structure (>4 levels) | Warning | Poor navigation UX, consider flattening |
| Orphaned assets (unreferenced images) | Warning | Bloats repository, may be leftover from deletions |
| Mixed file naming styles in same directory | Warning | Inconsistent developer experience |

**Warning Message Example:**
```
WARNING: Missing section index file
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

#### Asset Rules (Warnings - Allow but Notify)

| Rule | Severity | Rationale |
|------|----------|-----------|
| Image filename with spaces/uppercase | Warning | Works but creates inconsistent URLs |
| Absolute URLs to internal assets | Warning | Breaks in local development, not portable |
| Large binary files (>5MB) | Warning | Git performance, consider external hosting |
| Embedded diagram formats (`.drawio.png`, `.drawio.svg`) | Info | Valid double extension for editable diagrams, explicitly allowed |

### Implementation Phases

**Phase 1: Core Linting Engine (Week 1)**
- Implement `internal/lint` package with rule engine
- Filename validation (errors only)
- Human-readable formatter
- Unit tests for each rule

**Phase 2: CLI and Manual Workflow (Week 1-2)**
- Add `docbuilder lint` command
- Intelligent default path detection (`docs/` or `documentation/`)
- Support single file, directory, and recursive modes
- Exit codes: 0 (clean), 1 (warnings), 2 (errors)
- Colorized terminal output (red errors, yellow warnings)

**Phase 3: Auto-Fix Capability (Week 2)**
- Implement safe file renaming with link resolution:
  - Rename files: `My Doc.md` → `my-doc.md`
  - Scan all markdown files for links to renamed files
  - Update internal references preserving link style (relative/absolute)
  - Handle image links, inline links, reference-style links
  - Preserve anchor fragments (#section) in links
  - Preserve Git history (using `git mv`)
- Require `--fix` flag and confirmation prompt showing:
  - Files to be renamed
  - Markdown files that will be updated
  - Total number of links to be modified
- Dry-run mode: `--fix --dry-run` shows all changes without applying
- Generate detailed fix report with before/after comparison

**Phase 4: Integration Hooks (Week 3)**
- Traditional pre-commit hook script (`scripts/install-hooks.sh`)
- Lefthook configuration (`lefthook.yml`)
- GitHub Actions workflow example
- GitLab CI template
- Documentation in `docs/how-to/setup-linting.md`

**Phase 5: Content and Structure Rules (Future)**
- Frontmatter validation
- Link checking
- Orphaned asset detection
- Structure recommendations

### Lint Command Interface

**Default Behavior:**
When run without arguments, `docbuilder lint` uses intelligent path detection:
1. If `docs/` directory exists in current directory → lint `docs/`
2. If `documentation/` directory exists → lint `documentation/`
3. Otherwise → lint current directory (`.`)

```bash
# Lint with intelligent defaults (checks for docs/ or documentation/)
docbuilder lint

# Lint current directory explicitly
docbuilder lint .

# Lint specific path
docbuilder lint ./docs

# Lint with auto-fix (prompts for confirmation)
docbuilder lint --fix

# Lint with auto-fix without confirmation (CI mode)
docbuilder lint --fix --yes

# Dry-run to see what would be fixed
docbuilder lint --fix --dry-run

# Quiet mode (errors only, no warnings)
docbuilder lint --quiet

# JSON output for CI integration
docbuilder lint --format=json

# Show detailed explanations for rules
docbuilder lint --explain
```

### Exit Codes

- `0`: No issues found (clean)
- `1`: Warnings present but no errors
- `2`: Errors found (build would fail)
- `3`: Lint execution error (filesystem access, etc.)

### Output Format

```
Linting documentation in: ./docs
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✗ docs/API Guide.md
  ERROR: Invalid filename
  └─ Contains uppercase letters and spaces
  
  Current:  docs/API Guide.md
  Suggested: docs/api-guide.md
  
  Why: Spaces create %20 in URLs; uppercase causes 
  case-sensitivity issues across platforms.
  
  Fix: docbuilder lint --fix docs/

⚠ docs/api/_index.md
  WARNING: Missing section title
  └─ Frontmatter has no title field
  
  Add title to frontmatter:
  ---
  title: "API Documentation"
  ---

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results:
  2 files scanned
  1 error (blocks build)
  1 warning (should fix)
  
Exit code: 2
```

### Safety Guarantees for Auto-Fix

The `--fix` flag will **only** perform transformations that are provably safe:

1. **Filename normalization**: Lowercase + hyphenate, preserving whitelisted double extensions (reversible)
2. **Link updates**: Update relative links in same repo (validated before commit)
3. **Git integration**: Use `git mv` to preserve history
4. **Atomic operations**: All-or-nothing (rollback on any failure)
5. **Backup prompt**: Confirms user wants to proceed
6. **Dry-run first**: Shows changes before applying

### Default Path Detection

To minimize friction, `docbuilder lint` intelligently detects documentation directories:

**Detection Order:**
1. Check for `docs/` directory in current path
2. Check for `documentation/` directory in current path
3. Fallback to current directory (`.`)

**Override Behavior:**
- Explicit path argument always takes precedence: `docbuilder lint ./custom-docs`
- Use `docbuilder lint .` to explicitly lint current directory

**Rationale:**
- Most projects use `docs/` or `documentation/` as standard convention
- Reduces cognitive load for developers (just run `docbuilder lint`)
- Follows principle of least surprise
- Works naturally in CI/CD where working directory is project root

**Will NOT auto-fix:**
- Frontmatter structure (too complex, context-dependent)
- External links (can't validate without network)
- Content rewrites (subjective)
- Cross-repository links (affects multiple repos)

### Git Hooks Integration

#### Option 1: Lefthook (Recommended)

Lefthook is a fast, modern Git hooks manager. Add to `lefthook.yml` in repository root:

```yaml
# lefthook.yml
pre-commit:
  parallel: true
  commands:
    lint-docs:
      glob: "*.{md,markdown,png,jpg,jpeg,gif,svg}"
      run: docbuilder lint {staged_files} --quiet
      stage_fixed: true  # Auto-stage files fixed with --fix flag
      
    # Optional: auto-fix on commit
    lint-docs-fix:
      glob: "*.{md,markdown,png,jpg,jpeg,gif,svg}"
      run: docbuilder lint {staged_files} --fix --yes
      stage_fixed: true
```

**Installation:**
```bash
# Install lefthook (one-time setup)
brew install lefthook  # macOS
# or
go install github.com/evilmartians/lefthook@latest

# Install hooks in repository
lefthook install
```

**Benefits:**
- Fast parallel execution
- Easier to configure and maintain
- Portable configuration (checked into repo)
- Supports multiple hooks and commands
- Auto-staging of fixed files

#### Option 2: Traditional Pre-Commit Hook

Install via: `docbuilder lint install-hook`

Generated hook at `.git/hooks/pre-commit`:
```bash
#!/bin/sh
# DocBuilder documentation linting pre-commit hook

# Only lint staged markdown and asset files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep -E '\.(md|markdown|png|jpg|jpeg|gif|svg)$')

if [ -n "$STAGED_FILES" ]; then
  echo "Linting staged documentation files..."
  docbuilder lint $STAGED_FILES --quiet
  
  LINT_EXIT=$?
  
  if [ $LINT_EXIT -eq 2 ]; then
    echo ""
    echo "ERROR: Lint errors found. Commit blocked."
    echo "Fix errors or run: docbuilder lint --fix"
    exit 1
  elif [ $LINT_EXIT -eq 1 ]; then
    echo ""
    echo "WARNING:  Lint warnings present. Consider fixing before commit."
    echo "To auto-fix: docbuilder lint --fix"
    # Allow commit but show warning
    exit 0
  fi
fi

exit 0
```

### CI/CD Integration

**GitHub Actions** (`.github/workflows/lint-docs.yml`):
```yaml
name: Lint Documentation
on: [pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install DocBuilder
        run: |
          curl -L https://github.com/org/docbuilder/releases/latest/download/docbuilder-linux-amd64 -o docbuilder
          chmod +x docbuilder
          sudo mv docbuilder /usr/local/bin/
      
      - name: Lint Documentation
        run: docbuilder lint ./docs --format=json > lint-report.json
      
      - name: Upload Report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: lint-report
          path: lint-report.json
      
      - name: Comment PR
        if: failure()
        uses: actions/github-script@v6
        with:
          script: |
            const report = require('./lint-report.json');
            // Post formatted comment with errors
```

**GitLab CI** (`.gitlab-ci.yml`):
```yaml
lint-docs:
  stage: test
  image: docbuilder:latest
  script:
    - docbuilder lint ./docs --format=json | tee lint-report.json
  artifacts:
    when: always
    reports:
      junit: lint-report.json
  allow_failure: false
```

## Auto-Fix Implementation: Link Resolution

When the `--fix` flag is used, renaming files requires updating all internal markdown links that reference those files. This is critical to prevent broken documentation after auto-fixing filename issues.

### Link Resolution Strategy

#### Supported Link Types

The fixer must handle all common markdown link patterns:

```markdown
<!-- 1. Inline links (most common) -->
[API Guide](API_Guide.md)
[API Guide](./API_Guide.md)
[API Guide](../docs/API_Guide.md)

<!-- 2. Absolute repository paths -->
[API Guide](/docs/API_Guide.md)

<!-- 3. Reference-style links -->
[API Guide][api-ref]
[api-ref]: API_Guide.md "API Documentation"

<!-- 4. Image links -->
![Architecture](Architecture_Diagram.png)
![Logo](./assets/Company_Logo.svg)

<!-- 5. Links with anchors (preserve fragment) -->
[Authentication](API_Guide.md#authentication)
[Overview](./API_Guide.md#overview)

<!-- 6. Links in code blocks (ignore) -->
```bash
# See API_Guide.md for details
```
```

#### Resolution Algorithm

**Phase 1: Discover Links**
```go
type LinkReference struct {
    SourceFile  string   // File containing the link
    LineNumber  int      // Line number of link
    LinkType    LinkType // Inline, Reference, Image
    Target      string   // Link target (path)
    Fragment    string   // Anchor fragment (#section)
    FullMatch   string   // Complete original text for replacement
}

func (f *Fixer) findLinksToFile(targetPath string) ([]LinkReference, error) {
    // 1. Walk all .md files in documentation directory
    // 2. For each file, scan for links using regex patterns:
    //    - Inline: \[([^\]]+)\]\(([^)]+)\)
    //    - Reference: ^\[([^\]]+)\]:\s*(.+?)(?:\s+"([^"]*)")?$
    //    - Image: !\[([^\]]*)\]\(([^)]+)\)
    // 3. Parse link target to extract path and fragment
    // 4. Resolve relative paths to absolute workspace paths
    // 5. Compare resolved path with target file
    // 6. Collect all matches as LinkReference structs
}
```

**Phase 2: Path Resolution**
```go
func resolveRelativePath(sourceFile, linkTarget string) (string, error) {
    // Given: sourceFile = "docs/guides/tutorial.md"
    //        linkTarget = "../api/API_Guide.md"
    // 
    // 1. Get directory of source file: "docs/guides/"
    // 2. Join with link target: "docs/guides/../api/API_Guide.md"
    // 3. Clean path: "docs/api/API_Guide.md"
    // 4. Resolve to absolute workspace path
    // 5. Return canonical path for comparison
}
```

**Phase 3: Generate Replacement**
```go
func (f *Fixer) updateLink(ref LinkReference, oldPath, newPath string) string {
    // Preserve link style (relative vs absolute)
    // Update filename while keeping directory structure
    // Preserve anchor fragments
    // Preserve reference link titles
    
    // Example:
    // Original: [API](../api/API_Guide.md#auth)
    // Old path: docs/api/API_Guide.md
    // New path: docs/api/api-guide.md
    // Result:   [API](../api/api-guide.md#auth)
}
```

**Phase 4: Apply Updates**
```go
func (f *Fixer) applyLinkUpdates(updates []LinkUpdate) error {
    // 1. Group updates by source file
    // 2. For each file, apply all updates atomically:
    //    - Read file content
    //    - Apply replacements (in reverse line order to preserve offsets)
    //    - Write updated content
    // 3. If any update fails, rollback all changes
    // 4. Generate fix report showing what was updated
}
```

### Edge Cases and Safety

**Case 1: External URLs**
```markdown
[GitHub Docs](https://github.com/docs/API_Guide.md)
```
**Resolution:** Skip. Only update links to local files. Detect by checking for protocol scheme (`http://`, `https://`).

**Case 2: Broken Links**
```markdown
[Old Guide](Deleted_File.md)
```
**Resolution:** If link target doesn't exist and matches old filename pattern, report separately as "potential broken link" but don't update.

**Case 3: Multiple Files Same Name**
```markdown
docs/api/README.md
docs/guides/README.md
```
**Resolution:** Use full path matching. Only update links that resolve to the specific file being renamed.

**Case 4: Circular References**
```markdown
<!-- File A.md -->
[See B](B.md)

<!-- File B.md -->
[See A](A.md)
```
**Resolution:** No special handling needed. Each file rename updates its own references independently.

**Case 5: Links in Code Blocks**
````markdown
```bash
# Download API_Guide.md from repository
curl https://example.com/API_Guide.md
```
````
**Resolution:** Don't update links inside code blocks. Use markdown parser to identify fenced code blocks and skip them.

**Case 6: Case-Insensitive Filesystems**
```markdown
[Guide](api_guide.md)  # Links to API_Guide.md on macOS/Windows
```
**Resolution:** Perform case-insensitive path comparison when checking if link targets the file being renamed.

### User Confirmation Flow

When `--fix` flag is used without `--yes`, show interactive confirmation:

```
Found 3 files with naming issues:

Files to rename:
  1. docs/API_Guide.md → docs/api-guide.md
  2. docs/User Manual.md → docs/user-manual.md
  3. images/Company_Logo.png → images/company-logo.png

Links to update:
  • docs/index.md (2 links)
  • docs/guides/getting-started.md (1 link)
  • docs/tutorials/quickstart.md (4 links)
  
Total: 7 links in 3 files will be updated

This will:
  ✓ Rename 3 files using git mv (preserves history)
  ✓ Update 7 internal links in 3 markdown files
  ✓ Create backup: .docbuilder-backup-20251229-143052/

Proceed with fixes? [y/N]: _
```

### Dry-Run Output

`docbuilder lint --fix --dry-run` shows what would change without applying:

```
DRY RUN: No changes will be applied

[File Renames]
  docs/API_Guide.md → docs/api-guide.md

[Link Updates]
  docs/index.md:12
    Before: [API Guide](API_Guide.md)
    After:  [API Guide](api-guide.md)
  
  docs/index.md:45
    Before: ![Diagram](../images/Architecture_Diagram.png)
    After:  ![Diagram](../images/architecture-diagram.png)
  
  docs/guides/getting-started.md:8
    Before: See the [API Guide](../API_Guide.md#authentication) for details.
    After:  See the [API Guide](../api-guide.md#authentication) for details.

Summary:
  3 files would be renamed
  7 links would be updated across 3 files
```

### Implementation Phases

**Phase 3a: Basic Renaming**
- File rename with `git mv` support
- Confirmation prompts
- Dry-run mode

**Phase 3b: Link Discovery**
- Scan markdown files for links
- Parse inline, reference, and image links
- Resolve relative paths

**Phase 3c: Link Updates**
- Generate replacement text
- Apply updates atomically
- Rollback on failure

**Phase 3d: Edge Cases**
- Skip external URLs
- Handle code blocks
- Case-insensitive matching

**Phase 3e: Reporting**
- Detailed fix report
- Dry-run preview
- Interactive confirmation

## Testing Strategy

Follow ADR-001 golden testing approach:

```
test/
  testdata/
    lint/
      valid/
        correct-filenames/      # All lowercase, hyphens
        proper-frontmatter/     # Valid YAML
        correct-links/          # All internal links valid
        whitelisted-extensions/ # .drawio.png, .drawio.svg
      
      invalid/
        mixed-case/             # MixedCase.md files
        spaces/                 # My Document.md
        special-chars/          # file@name.md, file#tag.md
        bad-frontmatter/        # Malformed YAML
        broken-links/           # Links to non-existent files
        invalid-double-ext/     # .md.backup, .markdown.old
      
      fix/
        links/
          # Test files for link resolution
          before/
            docs/
              API_Guide.md            # File to be renamed
              index.md                # Contains link to API_Guide.md
              guides/
                tutorial.md           # Contains relative link ../API_Guide.md
            images/
              Diagram.png             # Image to be renamed
          after/
            # Expected state after --fix applied
            docs/
              api-guide.md            # Renamed file
              index.md                # Link updated to api-guide.md
              guides/
                tutorial.md           # Link updated to ../api-guide.md
            images/
              diagram.png             # Renamed image
      
    golden/
      mixed-case.golden.json          # Expected error output
      spaces.golden.json              # Expected error output
      drawio-allowed.golden.json      # Verify .drawio.* passes
      fix-with-links.golden.json      # Expected fix report with link updates
      fix-dry-run.golden.txt          # Expected dry-run preview output
```

**Test Coverage:**
- Each rule has unit test with valid/invalid cases
- Integration tests run linter on test directories
- Golden files verify exact error messages
- **Auto-fix tests verify safe transformations**
  - Test file renaming with git mv
  - Test link discovery and resolution
  - Test link updates preserve style (relative/absolute)
  - Test anchor fragments are preserved
  - Test external URLs are not modified
  - Test links in code blocks are ignored
  - Test rollback on failure
- Pre-commit hook tested via Git test repository
- Default path detection tested (docs/, documentation/, fallback)
- **Link resolution tests:**
  - Unit tests for path resolution (relative → absolute)
  - Unit tests for link regex patterns (inline, reference, image)
  - Integration tests with before/after directory structures
  - Edge case tests (external URLs, code blocks, broken links)
  - Case-insensitive filesystem tests

## Keeping Linting Rules Synchronized with DocBuilder

The linting system must stay synchronized with DocBuilder's actual behavior to remain useful. As DocBuilder evolves—adding new features or changing how it processes documentation—the linter must reflect these changes.

### Synchronization Strategy

**1. Shared Test Infrastructure**

Linting rules should be validated against **actual DocBuilder behavior**, not assumptions:

```go
// Test that linter rejects what DocBuilder would fail to build
func TestLintRejectsInvalidFiles(t *testing.T) {
    invalidFiles := []string{
        "My Document.md",      // Spaces
        "API_Guide.md",        // Uppercase
        "file@special.md",     // Special chars
    }
    
    for _, file := range invalidFiles {
        // Verify linter catches it
        result := linter.LintFile(file)
        require.True(t, result.HasErrors())
        
        // Verify DocBuilder would actually fail/warn
        buildResult := docbuilder.BuildWithFile(file)
        require.False(t, buildResult.Success())
    }
}
```

**2. Integration Tests with Full Pipeline**

Periodically run integration tests that:
- Create test repositories with various violations
- Run `docbuilder build` on them
- Verify linter warnings/errors match actual build issues
- Catch cases where linter is too strict or too lenient

Example test structure:
```
test/integration/
  lint_docbuilder_sync_test.go  # Tests linter matches build behavior
```

**3. Version Alignment**

Linting rules should evolve with DocBuilder versions:

| DocBuilder Version | Linter Rule Changes |
|--------------------|---------------------|
| 1.0 - 1.5 | Basic filename and frontmatter rules |
| 1.6+ | Enhanced frontmatter schema validation |
| 2.0+ | Asset transformation and link validation |
| Future | Custom Hugo module support detection |

**Version compatibility approach:**
- Linter reports its "target DocBuilder version"
- Warns if linting against much older/newer DocBuilder behavior
- Can optionally validate against multiple versions

**4. Feature Detection**

When DocBuilder adds new features, update linter rules accordingly:

| DocBuilder Feature | Linting Rule Update |
|--------------------|---------------------|
| New frontmatter field support | Add validation for new fields |
| Asset transformation (WebP) | Allow new file extensions |
| Custom shortcodes | Validate shortcode syntax |
| Multi-language support | Validate language-specific paths |
| Repository metadata injection | Validate editURL patterns |

**5. Documentation Cross-References**

Maintain bidirectional links between linter rules and DocBuilder documentation:

```markdown
<!-- In lint error message -->
ERROR: Invalid filename contains uppercase letters
Learn more: https://docs.docbuilder.io/reference/filename-conventions

<!-- In DocBuilder docs -->
## Filename Conventions
DocBuilder expects lowercase, hyphenated filenames.
Use `docbuilder lint` to validate: https://docs.docbuilder.io/how-to/setup-linting
```

**6. Automated Sync Checks**

Add CI checks to prevent drift:

```yaml
# .github/workflows/lint-sync-check.yml
name: Lint Rules Sync Check
on: [pull_request]

jobs:
  verify-sync:
    runs-on: ubuntu-latest
    steps:
      - name: Run sync tests
        run: go test ./test/integration/lint_docbuilder_sync_test.go -v
      
      - name: Check for new DocBuilder features
        run: |
          # Parse recent commits for new features
          # Check if corresponding lint rules exist
          ./scripts/check-lint-coverage.sh
```

**7. Maintenance Workflow**

When DocBuilder changes:

1. **Feature Addition**: 
   - Update linter to recognize new valid patterns
   - Add test cases for new feature
   - Update `docs/reference/lint-rules.md`

2. **Deprecation**:
   - Linter warns about deprecated patterns
   - Provide migration suggestions
   - Eventually promote warnings to errors

3. **Bug Fixes**:
   - If DocBuilder now accepts something it previously rejected, update linter
   - Add regression test
   - Update golden test files

**8. Rule Evolution Process**

```
DocBuilder Change → Update Lint Rule → Add Tests → Update Docs → Release Notes
        ↓              ↓                  ↓            ↓             ↓
   feat: support   Add .webp to      Golden test   Update rule   v1.7.0:
   WebP images    allowed assets     for WebP      reference     WebP support
```

**9. Feedback Loop**

Monitor false positives/negatives:
- Track GitHub issues tagged `linter-false-positive` or `linter-missed-issue`
- Periodic review of linter vs actual build failures
- User feedback in success metrics (see Success Metrics section)

**10. Living Documentation**

Maintain a changelog specifically for linting rules:

```markdown
# Linting Rules Changelog

## v1.7.0 (2026-01-15)
- Added: Support for .webp images (DocBuilder v1.7.0)
- Changed: .drawio.png now explicitly whitelisted (was implicitly allowed)
- Fixed: False positive on _index.md in root directory

## v1.6.0 (2025-12-29)
- Initial release: Filename and frontmatter validation
```

### Practical Example: Adding Asset Transformation Support

**Scenario**: DocBuilder adds support for automatic WebP image conversion.

**Synchronization steps:**

1. **Detect the change**: PR adds WebP transformation to `internal/docs/assets.go`

2. **Update linter rules**:
```go
// internal/lint/asset_rules.go
func (l *Linter) validateAssetFile(file string) []Issue {
    ext := filepath.Ext(file)
    allowedExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"}
    
    if !contains(allowedExts, ext) {
        return []Issue{{
            Severity: Error,
            Message:  fmt.Sprintf("Unsupported image format: %s", ext),
        }}
    }
    return nil
}
```

3. **Add tests**:
```go
func TestWebPAssetValidation(t *testing.T) {
    linter := NewLinter()
    result := linter.LintFile("diagram.webp")
    require.False(t, result.HasErrors(), "WebP should be allowed")
}
```

4. **Update documentation**:
   - `docs/reference/lint-rules.md`: Add WebP to allowed asset formats
   - Update asset rules table with WebP examples

5. **Release together**: Linter v1.8.0 released alongside DocBuilder v1.8.0

### Ownership and Responsibility

- **Core team**: Maintains synchronization, reviews PRs for drift
- **Feature developers**: Update linter rules when adding DocBuilder features
- **PR checklist**: "Have you updated linting rules if applicable?"
- **Quarterly review**: Check for accumulated drift, plan alignment work

## Consequences

### Positive

1. **Early feedback**: Developers catch issues before commit
2. **Consistent quality**: Opinionated rules enforce best practices
3. **Better documentation**: Improved structure and linking
4. **Faster CI**: Fewer build failures from preventable issues
5. **Self-documenting**: Error messages teach Hugo conventions
6. **Safe automation**: `--fix` flag reduces manual renaming work
7. **Zero configuration**: Intelligent defaults work out of the box (auto-detects `docs/`)

### Negative

1. **Initial friction**: Existing repositories may have many violations
2. **Migration effort**: Teams must fix legacy documentation
3. **Learning curve**: Developers learn new rules
4. **Hook conflicts**: May conflict with other pre-commit tools

### Mitigation

- **Gradual rollout**: Start with warnings, move to errors over time
- **Migration guide**: Document bulk-fixing existing repositories
- **Rule documentation**: Comprehensive explanation of each rule
- **Opt-in initially**: Teams adopt voluntarily before enforcement

## Migration Path

### Week 1: Soft Launch

- Release `docbuilder lint` command (warnings only)
- Documentation in `docs/how-to/`
- Encourage voluntary adoption

### Week 2: Team Testing

- Select 2-3 pilot repositories
- Run `docbuilder lint --fix` to clean up
- Gather feedback on rules and messages

### Week 3: Git Hooks

- Publish traditional hook installer
- Add lefthook.yml to repository template
- Provide team-wide installation guide (both options)
- Keep as warnings (non-blocking)

### Month 2: CI Integration

- Add CI workflow to template repositories
- Start blocking PRs with errors (not warnings)
- Monitor false positives, adjust rules if needed

### Month 3: Full Enforcement

- All repositories have lint checks
- Warnings promoted to errors where appropriate
- Legacy repositories cleaned up or exempted

## Future Enhancements

### Content Linting (Phase 5)

- Spell checking (en-US by default, configurable)
- Markdown style consistency (headings, lists, code blocks)
- Accessibility checks (alt text, heading hierarchy)
- SEO recommendations (meta descriptions, keywords)

### Advanced Asset Handling

- Accessibility score for images (alt text quality)

### IDE Integration

- VS Code extension for real-time linting
- Language server protocol (LSP) for any editor
- Inline quick-fixes and refactorings

### Smart Fixes

- Automatic frontmatter generation from content
- Link suggestion for orphaned sections
- Batch rename with preview

## Examples

### Example 1: Clean Repository (Using Defaults)
```bash
# Run from project root - automatically detects docs/
$ docbuilder lint

Detected documentation directory: docs/
Linting documentation in: ./docs
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ docs/_index.md
✓ docs/getting-started.md
✓ docs/api/authentication.md
✓ docs/api/_index.md
✓ docs/images/architecture-diagram.drawio.png (whitelisted double extension)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results:
  5 files scanned
  0 errors
  0 warnings
  
✨ All documentation passes linting!

Exit code: 0
```

### Example 1b: No docs/ Directory Found
```bash
# Run from project root without docs/ directory
$ docbuilder lint

No documentation directory detected (checked: docs/, documentation/)
Falling back to current directory: .

Linting documentation in: .
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ README.md
✓ CONTRIBUTING.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results:
  2 files scanned
  0 errors
  0 warnings

Exit code: 0

# To lint a custom directory, specify it explicitly
$ docbuilder lint ./custom-docs
```

### Example 2: Filename Issues
```bash
$ docbuilder lint ./docs

Linting documentation in: ./docs
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✗ docs/API Guide.md
  ERROR: Invalid filename
  └─ Contains uppercase letters (A, P, I, G) and space character
  
  Current:  docs/API Guide.md
  Suggested: docs/api-guide.md
  
  Why this matters:
    • Spaces become %20 in URLs: /docs/API%20Guide/
    • Mixed case causes issues on case-sensitive systems
    • Hugo expects lowercase, hyphenated slugs
  
  How to fix:
    1. Manual: mv "docs/API Guide.md" docs/api-guide.md
    2. Automatic: docbuilder lint --fix docs/
    
  If this file is linked from other docs, those links 
  will be automatically updated when using --fix.

✗ docs/screenshots/Login Screen.png
  ERROR: Invalid filename
  └─ Contains uppercase letters and space character
  
  Current:  docs/screenshots/Login Screen.png
  Suggested: docs/screenshots/login-screen.png
  
  Asset files follow the same rules as markdown files.
  Image references in markdown will be updated automatically.

✗ docs/architecture.md.backup
  ERROR: Invalid double extension
  └─ Contains non-whitelisted double extension .md.backup
  
  Current:  docs/architecture.md.backup
  Issue: Hugo will attempt to process this as markdown
  
  Whitelisted double extensions (allowed):
    • .drawio.png (Draw.io embedded PNG diagrams)
    • .drawio.svg (Draw.io embedded SVG diagrams)
  
  How to fix:
    1. Remove backup files from docs directory
    2. Use .git history or separate backup location
    3. Add to .gitignore: *.backup

✓ docs/diagrams/system-flow.drawio.svg
  INFO: Whitelisted double extension
  └─ .drawio.svg is explicitly allowed for embedded diagrams

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results:
  4 files scanned
  3 errors (blocks build)
  0 warnings
  1 info (explicitly allowed)
  
ERROR: Documentation has errors that will prevent Hugo build.
   Run: docbuilder lint --fix docs/

Exit code: 2
```

### Example 3: Auto-Fix Dry Run
```bash
$ docbuilder lint --fix --dry-run ./docs

Linting documentation in: ./docs (dry-run mode)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

The following changes would be made:

FILE RENAMES:
  docs/API Guide.md → docs/api-guide.md
  docs/screenshots/Login Screen.png → docs/screenshots/login-screen.png

CONTENT UPDATES:
  docs/usage.md
    Line 15: ![Screenshot](./screenshots/Login Screen.png)
         →   ![Screenshot](./screenshots/login-screen.png)
  
  docs/index.md
    Line 8: [API Guide](./API Guide.md)
         →  [API Guide](./api-guide.md)

GIT OPERATIONS:
  git mv "docs/API Guide.md" "docs/api-guide.md"
  git mv "docs/screenshots/Login Screen.png" "docs/screenshots/login-screen.png"
  git add docs/usage.md docs/index.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Summary:
  2 files would be renamed
  2 files would have content updated
  4 git operations would be performed

To apply these changes: docbuilder lint --fix ./docs

Exit code: 0
```

### Example 4: Lefthook Integration
```yaml
# lefthook.yml in repository root
pre-commit:
  parallel: true
  commands:
    lint-docs:
      glob: \"*.{md,markdown,png,jpg,jpeg,gif,svg}\"
      run: docbuilder lint {staged_files} --quiet
      fail_text: \"Documentation linting failed. Run 'docbuilder lint --fix' to auto-fix.\"
```

**Usage:**
```bash
# Developer makes changes and commits
$ git add docs/API\ Guide.md docs/getting-started.md
$ git commit -m "Add API documentation"

Lefthook > pre-commit > lint-docs:

Linting 2 staged files...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✗ docs/API Guide.md
  ERROR: Invalid filename (contains spaces and uppercase)
  Suggested: docs/api-guide.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 1 error, 1 passed

Documentation linting failed. Run 'docbuilder lint --fix' to auto-fix.

# Developer runs auto-fix
$ docbuilder lint --fix docs/
Fixed: docs/API Guide.md → docs/api-guide.md
Updated 2 references in other files

# Commit again (now passes)
$ git add -A
$ git commit -m "Add API documentation"

Lefthook > pre-commit > lint-docs:
✓ All staged files pass linting

[main a1b2c3d] Add API documentation
 1 file changed, 50 insertions(+)
 create mode 100644 docs/api-guide.md
```

## References

- [Hugo URL Management](https://gohugo.io/content-management/urls/)
- [Hugo Content Organization](https://gohugo.io/content-management/organization/)
- [Git Pre-Commit Hooks](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks)
- [Lefthook Documentation](https://github.com/evilmartians/lefthook)
- [Markdown Best Practices](https://www.markdownguide.org/basic-syntax/)
- ADR-001: Golden Testing Strategy (for test approach)
- ADR-000: Uniform Error Handling (for error reporting)

## Implementation Checklist

### Phase 1: Core Linting Engine ✅
- [x] Create `internal/lint` package (5 files: types, rules, linter, formatter, tests)
- [x] Implement filename rules with whitelisted extensions (.drawio.png, .drawio.svg)
- [x] Human-readable text formatter with colorization and NO_COLOR support
- [x] JSON formatter for CI/CD integration
- [x] Unit tests for each rule (11 comprehensive test cases)
- [x] Standard file filtering (README, CONTRIBUTING, CHANGELOG, etc.)
- [x] Intelligent default path detection (`docs/`, `documentation/`, fallback to `.`)

### Phase 2: CLI Implementation ✅
- [x] Add `docbuilder lint` CLI command with Kong integration
- [x] Exit code handling (0=clean, 1=warnings, 2=errors, 3=execution error)
- [x] Output format flags: `--format=text|json`
- [x] Verbosity control: `--quiet`, `--verbose`
- [x] Color detection with NO_COLOR environment variable
- [x] Duplicate error prevention (consolidated uppercase/special char reporting)

### Phase 3: Auto-Fix Capability (Link Resolution)
- [x] Comprehensive link resolution strategy documented in ADR
- [x] Phase 3a: Basic file renaming with git mv support
  - [x] File rename implementation
  - [x] Git mv integration for history preservation
  - [x] Dry-run mode (`--fix --dry-run`)
  - [x] Force flag for overwriting existing files
  - [x] Comprehensive test coverage (8 tests)
  - [ ] Interactive confirmation prompts (deferred to Phase 3e)
  - [ ] Detailed preview of changes (deferred to Phase 3e)
- [x] Phase 3b: Link discovery and path resolution
  - [x] Regex patterns for inline, reference, image links
  - [x] Relative path resolution to absolute workspace paths
  - [x] Link reference tracking (source file, line number, type)
  - [x] External URL detection and exclusion
  - [x] Code block detection and exclusion
  - [x] Anchor fragment preservation
  - [x] Comprehensive test coverage (13 tests, 379 lines)
- [x] Phase 3c: Link updates with atomic operations
  - [x] Generate replacement text preserving style
  - [x] Atomic file updates with rollback on failure
  - [x] Preserve anchor fragments (#section) in updated links
  - [x] Test coverage for anchor fragment preservation
  - [x] Test coverage for rollback mechanism
- [ ] Phase 3d: Edge case handling
  - [x] Skip external URLs (protocol detection) - already implemented in Phase 3b
  - [x] Ignore links in code blocks (markdown parser) - already implemented in Phase 3b
  - [x] Case-insensitive filesystem support
  - [x] Broken link detection and reporting
- [x] Phase 3e: Reporting and interactive confirmation
  - [x] Detailed fix report with statistics
  - [x] Interactive confirmation showing files + links affected
  - [x] Dry-run preview with before/after comparison
  - [x] Backup creation (.docbuilder-backup-{timestamp}/)

### Phase 4: Git Hooks Integration
- [x] Traditional pre-commit hook script (`scripts/install-hooks.sh`)
- [x] Hook installer command: `docbuilder lint install-hook`
- [x] Lefthook configuration example (`lefthook.yml`)
- [x] Test with staged files workflow

### Phase 5: CI/CD Integration
- [x] GitHub Actions workflow example (`.github/workflows/lint-docs.yml`)
- [x] GitLab CI template (`.gitlab-ci.yml`)
- [x] JSON output schema documentation
- [x] PR comment integration examples

### Testing
- [x] Integration tests with golden files for core lint functionality
  - [x] Valid scenarios: correct filenames, whitelisted extensions
  - [x] Invalid scenarios: mixed-case, spaces, special chars, double extensions
  - [x] Golden file generation with `-update-golden` flag
  - [x] Normalized path comparison for system-independence
  - [x] 6 comprehensive test cases covering all current rules
- [x] Integration golden tests for auto-fix functionality (Phase 3)
  - [x] Before/after directory structures with realistic test data
  - [x] TestGoldenAutoFix_FileRenameWithLinkUpdates: Complete fix workflow
  - [x] TestGoldenAutoFix_DryRun: Dry-run mode output verification
  - [x] TestGoldenAutoFix_BrokenLinkDetection: Broken link reporting
  - [x] Sorted results for consistent comparison across runs
  - [x] Normalized paths (filenames only) for portability
- [x] Integration tests for lint-DocBuilder sync
  - [x] TestLintDocBuilderSync: Full build pipeline → lint validation
  - [x] TestLintDocBuilderSync_FileNaming: Filename convention compliance
  - [x] Test repository with cross-reference links (./relative-link.md syntax)
  - [x] Link transformation bug fixes (strip ./ prefix in transform_links.go)
  - [x] Linter path resolution enhancements (Hugo site-absolute paths in fixer.go)
- [x] CI workflow to detect rule drift
  - [x] GitHub Actions workflow: .github/workflows/detect-rule-drift.yml
  - [x] Weekly schedule (Sunday midnight) + manual dispatch
  - [x] Single theme testing (Relearn only)
  - [x] Artifact uploads (90-day retention)
  - [x] PR comment integration on drift detection

### Documentation
- [x] `docs/how-to/setup-linting.md` - Setup and usage guide (completed)
- [x] `docs/reference/lint-rules.md` - Complete rule reference (completed)
- [x] `docs/reference/lint-rules-changelog.md` - Rule version history (completed)
- [x] `docs/how-to/migrate-to-linting.md` - Migration guide for existing repositories (completed)
- [x] `docs/how-to/ci-cd-linting.md` - CI/CD integration examples (completed)

### Future Enhancements
- [ ] VS Code extension for real-time linting
- [ ] Content linting rules (spell checking, style consistency)
- [ ] Advanced asset handling (accessibility checks)

## Success Metrics

After 3 months of deployment:
- [ ] 90%+ of commits pass linting without errors
- [ ] 50% reduction in documentation-related CI failures
- [ ] Positive developer feedback (survey)
- [ ] <5% false positive rate on errors
- [ ] Active usage of `--fix` flag (telemetry)

---

**Decision Owner**: [To be assigned]  
**Stakeholders**: Development team, documentation maintainers, DevOps  
**Review Date**: 3 months after implementation
