---
aliases:
  - /_uid/138c1d38-5a96-4820-8a74-dbb45c94a0e3/
categories:
  - architecture
date: 2025-12-18T00:00:00Z
fingerprint: d6f1d74f5bdd59c20b6139da505245c1f85623e2a5611ff3e8d5044114d9fefa
lastmod: "2026-01-22"
status: proposed
tags:
  - adr
  - markdown
  - forges
  - content-processing
title: 'ADR-004: Forge-Specific Markdown Support'
uid: 138c1d38-5a96-4820-8a74-dbb45c94a0e3
---

# ADR-004: Forge-Specific Markdown Support

## Status

**PROPOSED** - Draft for discussion

## Context

DocBuilder aims to enable markdown files to work seamlessly both in their source forge (GitHub, GitLab, Forgejo) and in the rendered Hugo documentation site. However, forges support forge-specific markdown extensions that create references to forge resources:

### GitLab-Specific Markdown (GLFM)

GitLab provides extensive reference syntax ([GitLab Flavored Markdown](https://docs.gitlab.com/ee/user/markdown.html#gitlab-specific-references)):

| Reference Type | Syntax | Cross-Project | Example |
|---------------|--------|---------------|---------|
| Issue | `#123`, `GL-123`, `[issue:123]` | `namespace/project#123` | `#42` → Issue link |
| Merge Request | `!123` | `namespace/project!123` | `!17` → MR link |
| Snippet | `$123` | `namespace/project$123` | `$5` → Snippet link |
| Epic | `&123`, `[epic:123]` | `group/subgroup&123` | `&8` → Epic link |
| User | `@username` | n/a | `@alice` → User profile |
| Label | `~bug`, `~"feature request"` | `namespace/project~bug` | `~priority::high` |
| Milestone | `%v1.0` | `namespace/project%v1.0` | `%release-1.0` |
| Commit | `9ba12248` | `namespace/project@9ba12248` | Short SHA link |
| Alert | `^alert#123` | `namespace/project^alert#123` | Alert reference |
| Contact | `[contact:test@example.com]` | n/a | CRM contact |

### GitHub-Specific Markdown (GFM)

GitHub also has reference syntax:

| Reference Type | Syntax | Cross-Repo | Example |
|---------------|--------|------------|---------|
| Issue/PR | `#123` | `owner/repo#123` | `#42` |
| User | `@username` | n/a | `@octocat` |
| Team | `@org/team` | n/a | `@github/docs` |
| Commit | `SHA` | `owner/repo@SHA` | `a1b2c3d` |

### Forgejo/Gitea Markdown

Similar to GitHub with some extensions:

| Reference Type | Syntax | Cross-Repo | Example |
|---------------|--------|------------|---------|
| Issue/PR | `#123` | `owner/repo#123` | `#42` |
| User | `@username` | n/a | `@alice` |
| Commit | `SHA` | `owner/repo@SHA` | `abc123` |

### The Problem

**Multi-Forge Environments:**
- DocBuilder may aggregate docs from multiple forge instances (e.g., `gitlab-main`, `gitlab-secondary`, `github-public`)
- Each document knows which forge instance it came from via `page.File.Forge` (the forge identifier)
- All references in a document refer to that same forge instance
- Simple references (`#123`) and cross-project references (`other/repo#123`) both refer to the document's source forge

**Rendering Challenges:**
- Forge-specific syntax (`#123`, `!456`) is not standard markdown
- References must be converted to standard markdown links for Hugo
- Must preserve readability in both source forge and rendered docs

**User Goals:**
1. Write markdown once, works in both forge and docs site
2. Reference issues, PRs, users, etc. naturally
3. Support cross-repository references
4. Handle multiple forge instances correctly

## Decision

Implement a **multi-stage forge-specific markdown transform** in the content pipeline with the following components:

### 1. Reference Transforms

**Stage:** `StageTransform`

**Strategy:** Build one focused transformer at a time, test thoroughly, then move to the next.

**Implementation Approach:** One transformer per reference type per forge (detects pattern and builds URL in single pass)

```go
// GitLabIssueReferenceTransform handles GitLab issue references (#123)
// Detects patterns and builds URLs in a single pass
type GitLabIssueReferenceTransform struct {
    cache ReferenceCache
}

func (t *GitLabIssueReferenceTransform) CanTransform(page *ContentPage, ctx *TransformContext) bool {
    forgeConfig := ctx.Generator.GetConfig().GetForgeByName(page.File.Forge)
    return forgeConfig != nil && forgeConfig.Type == config.ForgeGitLab
}

func (t *GitLabIssueReferenceTransform) Transform(page *ContentPage, ctx *TransformContext) (*TransformationResult, error) {
    forgeConfig := ctx.Generator.GetConfig().GetForgeByName(page.File.Forge)
    
    // Detect issue patterns: #123, [issue:123], GL-123
    issuePattern := regexp.MustCompile(`(?:^|[\s(])(?:#(\d+)|GL-(\d+)|\[issue:(\d+)\])(?:[\s.,;:!?)]|$)`)
    
    // Find all matches with their positions
    matches := issuePattern.FindAllStringSubmatchIndex(page.Content, -1)
    if len(matches) == 0 {
        return NewTransformationResult().SetSuccess(), nil
    }
    
    // Process matches in reverse order to maintain string positions
    var replacements []replacement
    for _, match := range matches {
        issueNum := extractNumber(page.Content[match[0]:match[1]]) // helper to extract number
        
        // Build URL (check cache first)
        cacheKey := fmt.Sprintf("gitlab:issue:%s:%d", page.File.Repository, issueNum)
        url := t.cache.Get(cacheKey)
        if url == "" {
            url = fmt.Sprintf("%s/%s/-/issues/%d", 
                forgeConfig.BaseURL, 
                page.File.Repository, 
                issueNum)
            t.cache.Set(cacheKey, url)
        }
        
        // Create markdown link
        replacement := fmt.Sprintf("[#%d](%s)", issueNum, url)
        replacements = append(replacements, replacement{
            start: match[0],
            end:   match[1],
            text:  replacement,
        })
    }
    
    // Apply replacements in reverse order
    page.Content = applyReplacements(page.Content, replacements)
    return NewTransformationResult().SetSuccess(), nil
}

// GitLabMergeRequestReferenceDetector handles only GitLab MR references (!123)
type GitLabMergeRequestReferenceDetector struct {}

func (t *GitLabMergeRequestReferenceDetector) CanTransform(page *ContentPage, ctx *TransformContext) bool {
    forgeConfig := ctx.Generator.GetConfig().GetForgeByName(page.File.Forge)
    return forgeConfig != nil && forgeConfig.Type == config.ForgeGitLab
}

// GitLabLabelReferenceDetector handles only GitLab label references (~label)
type GitLabLabelReferenceDetector struct {}

// ... similar structure

// GitHubIssueReferenceDetector handles GitHub issue/PR references (#123)
type GitHubIssueReferenceDetector struct {}

// GitHubUserReferenceDetector handles GitHub user mentions (@username)
type GitHubUserReferenceDetector struct {}

// GitHubTeamReferenceDetector handles GitHub team mentions (@org/team)
type GitHubTeamReferenceDetector struct {}

// ... and so on for each reference type
```

**Pros:**
- Very focused, single-purpose transformers (single responsibility)
- Easy to test each reference type independently
- Simple to add new reference types
- Single pass per reference type (efficient)
- Easy to understand what each transformer does
- Direct pattern → URL conversion (no intermediate state)

**Cons:**
- Many transformers (could be 20+ total)
- More `CanTransform()` checks in pipeline (minimal performance impact)
- Some duplication of pattern matching logic (mitigated by helper utilities)

**Pipeline Registration:**

```go
func defaultTransforms(cfg *config.Config) []FileTransform {
    cache := NewReferenceCache() // Shared cache instance
    
    return []FileTransform{
        parseFrontMatter,
        normalizeIndexFiles,
        // ... existing transforms
        
        // GitLab reference transforms (detect + build URL in one pass)
        NewGitLabIssueReferenceTransform(cache),
        NewGitLabMergeRequestReferenceTransform(cache),
        NewGitLabLabelReferenceTransform(cache),
        NewGitLabMilestoneReferenceTransform(cache),
        NewGitLabSnippetReferenceTransform(cache),
        NewGitLabEpicReferenceTransform(cache),
        NewGitLabUserReferenceTransform(cache),
        
        // GitHub reference transforms
        NewGitHubIssueReferenceTransform(cache),
        NewGitHubUserReferenceTransform(cache),
        NewGitHubTeamReferenceTransform(cache),
        
        // Forgejo reference transforms
        NewForgejoIssueReferenceTransform(cache),
        NewForgejoUserReferenceTransform(cache),
        
        serializeDocument,
    }
}
```

**No Configuration Required:**

Transforms are always active for documents from matching forge types. The `CanTransform()` guard ensures each transformer only processes appropriate documents.

**Testing Benefits:**

```go
func TestGitLabIssueReferenceTransform_SimpleReference(t *testing.T) {
    cache := NewMockCache()
    transform := NewGitLabIssueReferenceTransform(cache)
    
    page := &models.ContentPage{
        Content: "Fixed in #123",
        File: docs.DocFile{Forge: "my-gitlab", Repository: "org/repo"},
    }
    
    result, err := transform.Transform(page, ctx)
    require.NoError(t, err)
    
    // Verify content was replaced with markdown link
    assert.Equal(t, "Fixed in [#123](https://gitlab.com/org/repo/-/issues/123)", page.Content)
}

func TestGitLabIssueReferenceTransform_CacheHit(t *testing.T) {
    cache := NewMockCache()
    cache.Set("gitlab:issue:org/repo:456", "https://cached-url.com")
    
    transform := NewGitLabIssueReferenceTransform(cache)
    page := &models.ContentPage{
        Content: "See GL-456",
        File: docs.DocFile{Forge: "my-gitlab", Repository: "org/repo"},
    }
    
    result, err := transform.Transform(page, ctx)
    require.NoError(t, err)
    
    // Verify cached URL was used in replacement
    assert.Equal(t, "See [GL-456](https://cached-url.com)", page.Content)
}

// Each reference type gets focused, isolated tests for pattern detection, URL building, AND content replacement
```

**Build incrementally:** Start with GitLab issue transform (#123), test thoroughly, then add GitLab merge requests (!123), then move to other forges and reference types.

#### Shared Helper Utilities

```go
// Helper type for managing replacements
type replacement struct {
    start int
    end   int
    text  string
}

// applyReplacements applies text replacements in reverse order to maintain positions
func applyReplacements(content string, replacements []replacement) string {
    // Sort by position (descending) to apply from end to start
    sort.Slice(replacements, func(i, j int) bool {
        return replacements[i].start > replacements[j].start
    })
    
    for _, r := range replacements {
        content = content[:r.start] + r.text + content[r.end:]
    }
    return content
}
```

**Rendering Strategy:** Each transform converts references to standard markdown links:

```markdown
<!-- Source in GitLab -->
See issue #123 for details.

<!-- After GitLabIssueReferenceTransform -->
See issue [#123](https://gitlab.com/org/repo/-/issues/123) for details.
```

**Why standard markdown links:**
- Simple, no Hugo shortcodes needed
- Works in all markdown renderers
- Preserves functionality (clickable links in both forge and docs)
- Easy to test and verify

### 2. Configuration Schema

**Per-Forge Settings:**

```yaml
forges:
  - name: gitlab-main
    type: gitlab
    base_url: https://gitlab.com
    api_url: https://gitlab.com/api/v4
```

**No Additional Configuration Required:**

Forge reference processing works automatically without user configuration. The cache layer uses sensible defaults (24h TTL) and automatically selects NATS KV when available in daemon mode, gracefully degrading to in-memory cache otherwise.

### 3. Implementation Stages

**Phase 1: Basic References (Minimal Viable)**
- Implement transforms for basic patterns: `#123`, `!123`
- Each transform detects, builds URL, and replaces content
- Cache URL building results with 24h TTL
- Add metrics for reference frequency

**Phase 2: Advanced Features**
- Cross-project references
- Label and milestone support
- Epic support (GitLab)
- Snippet support

## Consequences

### Positive

1. **Dual Compatibility**: Markdown works in both forge and docs
2. **Rich References**: Support forge-native syntax naturally
3. **Multi-Forge**: Handle multiple forge instances correctly
4. **Extensible**: Easy to add new reference types
5. **Graceful Degradation**: Failures preserve original text
6. **Testable**: Each reference type can be tested independently (Option B)
7. **Simple**: No configuration needed, transforms run when applicable

### Negative

1. **Complexity**: Adds transforms to pipeline
2. **Maintenance**: Must track forge markdown spec changes
3. **Future API Work**: May need forge API for validation/metadata (not in initial scope)

### Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Performance impact | Cache URL building results, `CanTransform()` guards prevent unnecessary work |
| Forge spec changes | Version detection, graceful fallbacks |
| Pattern ambiguity | Well-tested regex patterns, explicit word boundaries |

## Alternatives Considered

### Alternative 1: No Processing (Status Quo)

**Approach:** Leave forge references as-is, let them break in rendered docs.

**Rejected because:**
- Poor user experience in docs
- Defeats dual-compatibility goal
- No better than current state

### Alternative 2: Hugo-Only Processing

**Approach:** Use Hugo's markdown processing hooks.

**Rejected because:**
- Locks us into Hugo implementation
- Can't reuse with other static generators
- Less control over processing

### Alternative 3: Client-Side JavaScript

**Approach:** Detect and resolve references in browser.

**Rejected because:**
- Doesn't work in static exports
- Requires forge API access from client
- Performance issues
- SEO problems

## Examples

### Example 1: Simple Issue Reference

**Source (in GitLab):**
```markdown
# API Documentation

The authentication bug was fixed in #123.
```

**After Processing:**
```markdown
# API Documentation

The authentication bug was fixed in [#123](https://gitlab.com/myorg/api-docs/-/issues/123).
```

**Frontmatter Addition:**
```yaml
forge_references:
  - type: issue
    id: 123
    url: https://gitlab.com/myorg/api-docs/-/issues/123
    resolved: true
```

### Example 2: Cross-Project Reference

**Source (in GitLab):**
```markdown
See the design in myorg/design-system#45.
```

**After Processing:**
```markdown
See the design in {{< forge-ref type="issue" project="myorg/design-system" id="45" >}}myorg/design-system#45{{< /forge-ref >}}.
```

### Example 3: Multiple Forge Types

**Configuration:**
```yaml
forges:
  - name: gitlab-main
    type: gitlab
    base_url: https://gitlab.com
    
  - name: github-oss
    type: github
    base_url: https://github.com
    
repositories:
  - url: https://gitlab.com/myorg/internal-docs
    forge: gitlab-main
    
  - url: https://github.com/myorg/public-docs
    forge: github-oss
```

**Source (in gitlab-main repo):**
```markdown
Internal issue: #100
Public discussion: myorg/public-docs#50
```

**After Processing:**
```markdown
Internal issue: [#100](https://gitlab.com/myorg/internal-docs/-/issues/100)
Public discussion: [myorg/public-docs#50](https://github.com/myorg/public-docs/issues/50)
```

## Implementation Plan

### Phase 1: Foundation (Week 1)
- [ ] Create shared helper utilities (`applyReplacements`, pattern extraction)
- [ ] Create `ReferenceCache` interface and NATS implementation
- [ ] Add cache factory with graceful degradation logic
- [ ] Unit tests for helper utilities and cache layer

### Phase 2: GitLab Issue References (Week 1-2)
- [ ] Implement `GitLabIssueReferenceTransform` (single transform: detect + build URL + replace)
- [ ] Pattern matching for `#123`, `GL-123`, `[issue:123]`
- [ ] URL building with cache integration
- [ ] Unit tests (pattern detection, URL building, content replacement)
- [ ] Golden test with GitLab repo containing issue references
- [ ] Test thoroughly before moving to next reference type

### Phase 3: GitLab Merge Requests (Week 2-3)
- [ ] Implement `GitLabMergeRequestReferenceTransform` (!123 syntax)
- [ ] Pattern matching for `!123` and cross-project `namespace/project!123`
- [ ] URL building for merge requests
- [ ] Unit tests for MR-specific patterns
- [ ] Update golden tests to include MR references
- [ ] Verify no regressions in issue transform

### Phase 4: GitHub Support (Week 3-4)
- [ ] Implement `GitHubIssueReferenceTransform` (#123 for issues and PRs)
- [ ] Pattern matching for same-repo and cross-repo references
- [ ] URL building for GitHub issues/PRs
- [ ] Unit tests for GitHub-specific patterns
- [ ] Golden test with GitHub repo
- [ ] Test multi-forge scenarios (GitLab + GitHub repos)

### Phase 5: Additional Reference Types (Week 4-5)
- [ ] GitLab: Labels (~label), Milestones (%v1.0), Users (@username)
- [ ] GitHub: Users (@username), Teams (@org/team)
- [ ] Forgejo: Issues (#123), Users (@username)
- [ ] Build one transform at a time, test thoroughly
- [ ] Integration tests with all reference types combined

### Phase 6: Cross-Project References (Week 5-6)
- [ ] Implement cross-project pattern detection (`namespace/project#123`)
- [ ] Update all transforms to handle cross-project syntax
- [ ] Test cross-project URL building
- [ ] Golden tests with cross-project references
- [ ] Performance optimization (regex compilation, caching efficiency)
- [ ] Error handling improvements

### Phase 7: Advanced Features (Week 6-7)
- [ ] GitLab Snippets ($123), Epics (&123), Alerts (^alert#123)
- [ ] Commit SHA references (all forges)
- [ ] Quoted labels (`~"feature request"`)
- [ ] Documentation and user guide
- [ ] Migration guide for existing deployments

### Phase 8: Optional Enhancements (Week 7-8)
- [ ] API-based validation (optional, disabled by default)
- [ ] Fetch issue/MR titles for richer link text
- [ ] Metrics for reference processing (count by type, cache hit rate)
- [ ] Advanced caching strategies (pre-warming, TTL tuning)

## Open Questions

1. **Forge Type Detection:** Do we need to detect forge type from content?
   - **Answer:** No! We already know from `page.File.Forge`. Each document tracks its source forge.

2. **Caching Strategy:** Use NATS KV (like link verification) or local cache?
   - **Answer:** Use NATS KV automatically when available (daemon mode) with 24h default TTL. Degrade gracefully to in-memory cache if NATS unavailable. Log when degradation occurs. No user configuration needed.

3. **Private References:** How to handle references to private issues?
   - **Recommendation:** Skip resolution, preserve original text, log warning

4. **Reference Validation:** Should we validate that references exist?
   - **Recommendation:** Optional validation (off by default), log warnings

5. **Shortcode Library:** Which Hugo theme should host shortcodes?
   - **Recommendation:** Include in all themes, theme-agnostic design

6. **API Authentication:** Use same auth as git operations?
   - **Recommendation:** Yes, reuse existing forge auth config

## References

- [GitLab Flavored Markdown](https://docs.gitlab.com/ee/user/markdown.html)
- [GitHub Flavored Markdown](https://github.github.com/gfm/)
- [Forgejo Markdown](https://forgejo.org/docs/latest/user/markdown/)
- [ADR-002: In-Memory Content Pipeline](adr-002-in-memory-content-pipeline.md)
- [ADR-003: Fixed Transform Pipeline](adr-003-fixed-transform-pipeline.md)

## Decision Log

- **2025-12-18:** Initial proposal created with forge-specific markdown support concept
- **2025-12-18:** Clarified to use existing `page.File.Forge` metadata (no forge type detection needed)
- **2025-12-18:** Adopted per-reference-type transformers with `CanTransform()` guards for maximum modularity
- **2025-12-18:** Removed all user configuration (no enable/disable flags) - well-tested transforms always run
- **2025-12-18:** Simplified caching to automatic NATS KV with graceful degradation (24h default TTL, no user config)
- **2025-12-18:** Removed `TransformerConfiguration` - transformers are stateless or hold only cache reference
- **2025-12-18:** Combined detection and resolution into single-pass transforms (no separate renderer stage)
- **2025-12-18:** Transforms modify `page.Content` directly with standard markdown links (no intermediate metadata)
- **2025-12-18:** Finalized incremental implementation strategy: build one transform, test thoroughly, then next
- **TBD:** Team review and feedback
- **TBD:** Implementation start date
