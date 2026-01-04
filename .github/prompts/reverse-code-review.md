# Reverse Code Review

Perform a reverse code review to identify dead code, over-engineered patterns, or refactoring opportunities in the DocBuilder codebase.

## Verification Methodology

For EACH claim, you MUST:

### 1. Trace Actual Usage

- Search for function/type name across ALL Go files using `grep_search` or `semantic_search`
- Check struct field declarations and usage
- Check initialization in constructors
- Check method calls on types
- Check interface implementations (`var _ Interface`)
- Check callback/event registration
- Include production AND test files

### 2. Understand Design Rationale

- Read package documentation (`doc.go` files)
- Read function/type comments explaining WHY they exist
- Check for architectural constraints (import cycles, circular dependencies)
- Look for extensibility patterns (plugin systems, provider interfaces)
- Understand testing requirements (mock interfaces, dependency injection)

### 3. Provide Evidence

- Every claim MUST include `file:line` references
- Include actual code snippets showing usage
- Count exact usage locations
- Distinguish between "unused" and "extensibility point"

## What Constitutes Dead Code

**Dead code is code that:**
1. Not imported by any package
2. Not called by any function
3. Not stored in any struct field
4. Not registered as a callback/handler
5. Not implemented as an interface
6. Not used in tests AND not used in production

**NOT dead code:**
- Code with low usage count but serves extensibility
- Code that prevents import cycles
- Interfaces with single implementation (may enable testing/decoupling)
- Factory patterns that inject dependencies
- Code used only in specific modes (daemon, CLI, server)

## Common Pitfalls to Avoid

**DO NOT:**
- Rely solely on imports to determine usage
- Assume low usage count means code is unnecessary
- Count lines as a proxy for "is this needed"
- Ignore initialization code in constructors
- Skip checking test files for usage
- Criticize patterns without understanding constraints
- Make assumptions about callback/event-driven usage

**DO:**
- Search for actual function calls: `FunctionName(`
- Search for struct field usage: `.fieldName`
- Check initialization: `fieldName:`
- Read architectural comments explaining WHY
- Verify claims with multiple searches
- Consider "Why might this pattern exist?"
- Check daemon/server/CLI mode differences

## Analysis Structure

For each potentially problematic package:

### Package Information
```
Package: internal/example/
Purpose: [from doc.go or package comment]
Files: [count] files, [count] lines (excluding tests)
```

### Usage Verification
```
Exported Functions/Types:
- FunctionA: [count] usages in [file:line, file:line]
- FunctionB: [count] usages in [file:line]

Struct Fields:
- FieldX: initialized in [file:line], used in [file:line]

Callbacks/Events:
- HandlerY: registered in [file:line], called in [file:line]
```

### Design Rationale
```
Why This Pattern Exists:
- [Reason from code comments]
- [Architectural constraint]
- [Extensibility requirement]
```

### Conclusion
```
Status: [ACTIVELY USED | DEAD CODE | EXTENSIBILITY POINT]
Recommendation: [KEEP | REFACTOR | REMOVE]
Confidence: [HIGH | MEDIUM | LOW]
```

## Search Commands Pattern

Use these tools to verify usage:

```
# Search for function calls
grep_search: query="FunctionName(", isRegexp=false

# Search for type usage
grep_search: query="TypeName{", isRegexp=false

# Search for struct field usage
grep_search: query="\.fieldName", isRegexp=true

# Search for interface implementations
grep_search: query="var _ InterfaceName", isRegexp=false

# Semantic search for conceptual usage
semantic_search: query="authentication provider initialization"
```

## Verification Checklist

Before finalizing any "dead code" claim:

- [ ] Searched entire codebase for type/function name
- [ ] Checked all struct fields for this type
- [ ] Checked all constructors for initialization
- [ ] Checked for callback/event registration
- [ ] Checked for interface implementations
- [ ] Read package documentation
- [ ] Read function/type comments
- [ ] Understood architectural constraints
- [ ] Verified usage count with file:line references
- [ ] Considered extensibility requirements
- [ ] Checked test files for usage
- [ ] Checked daemon/server/CLI mode differences

## Reporting Format

### Executive Summary
- Total packages analyzed: [count]
- Packages with dead code: [count]
- Packages with refactoring opportunities: [count]
- Total lines of removable code: [count]
- Confidence level: [HIGH | MEDIUM | LOW]

### For Each Finding

**Package:** internal/example/

**Claim:** [One sentence description]

**Evidence:**
- Exported: [list of exported symbols]
- Production usage: [count] times at [file:line, file:line]
- Test usage: [count] times at [file:line]

**Design Rationale:**
- [Why this pattern exists based on code comments/docs]

**Recommendation:** [REMOVE | REFACTOR | KEEP]

**Confidence:** [HIGH | MEDIUM | LOW]
- HIGH: No usage found after thorough search
- MEDIUM: Low usage, unclear if intentional
- LOW: Unclear design intent, needs clarification

## Self-Check Questions

Before submitting analysis:

1. Did I search for actual function calls, not just imports?
2. Did I check struct field usage, not just declarations?
3. Did I read the package documentation?
4. Did I understand why patterns exist before criticizing?
5. Did I check initialization in constructors?
6. Did I verify callback/event registration?
7. Did I provide file:line evidence for every claim?
8. Did I distinguish "unused" from "extensibility point"?
9. Did I check test files for usage?
10. Would I stake my credibility on each claim?

If answer to ANY question is "no": MORE VERIFICATION NEEDED.

## Example: Correct Analysis

**Package:** internal/auth/

**Claim:** Package appears over-abstracted but serves legitimate extensibility purpose.

**Evidence:**
- Exported: `Manager`, `CreateAuth()`, provider interfaces
- Production usage: 1 location: `internal/git/auth.go:13`
- Architecture: Manager → Registry → Provider interface → 4 concrete providers
- Files: 8 files (355 lines excluding tests)

**Design Rationale:**
- Registry pattern enables dynamic provider registration
- Interface enables testing with mock providers
- Follows Open/Closed Principle for provider extensibility
- Comment in `manager.go:35`: "Package-level instance for convenience"

**Recommendation:** KEEP
- Low usage count is intentional (single auth entry point)
- Pattern enables future auth types (OAuth, cert-based)
- Interface enables testing and mocking

**Confidence:** HIGH

## Golden Rule

> Never claim code is dead/unnecessary without tracing actual usage through:
> 1. Function calls
> 2. Struct fields
> 3. Initialization
> 4. Callbacks
> 5. Interface implementations
> 6. Test usage
> 7. Design rationale

**When in doubt:** Verify twice, claim once.

## Historical Context

This prompt was created after a flawed analysis (2026-01-04) that made 5 major errors by only checking imports instead of actual usage. All claimed "dead" code was in production use.
