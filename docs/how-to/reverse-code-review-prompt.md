---
title: "Reverse Code Review Methodology Prompt"
date: 2026-01-04
category: how-to
tags:
  - code-review
  - methodology
  - ai-prompts
---

# Reverse Code Review Methodology Prompt

This document provides a structured prompt for conducting thorough reverse code reviews (analyzing from binary entry points backwards to find dead/over-engineered code).

## Prompt Template

Use this prompt when requesting a reverse code review:

---

**TASK**: Perform a reverse code review of this codebase to identify dead code, over-engineered patterns, or refactoring opportunities.

**CRITICAL REQUIREMENTS**:

### 1. Verification Methodology

For EACH claim you make, you MUST:

✅ **Trace Actual Usage**:
- [ ] Search for function/type name across ALL Go files (use grep with function name)
- [ ] Check struct field declarations (`grep "fieldName"`)
- [ ] Check initialization in constructors (`grep "NewXxx"`)
- [ ] Check method calls (`grep "\.MethodName("`)
- [ ] Check interface implementations (`grep "var _ Interface"`)
- [ ] Check callback/event registration
- [ ] Include production AND test files in search

✅ **Understand Design Rationale**:
- [ ] Read package documentation (`doc.go` files)
- [ ] Read function/type comments explaining WHY they exist
- [ ] Check for architectural constraints (import cycles, circular dependencies)
- [ ] Look for extensibility patterns (plugin systems, provider interfaces)
- [ ] Understand testing requirements (mock interfaces, dependency injection)

✅ **Provide Evidence**:
- [ ] Every claim MUST include `file:line` references
- [ ] Include actual code snippets showing usage
- [ ] Count exact usage locations (e.g., "used in 5 places: file1.go:23, file2.go:45...")
- [ ] Distinguish between "unused" and "extensibility point"

### 2. What Constitutes Dead Code

**Dead code is code that is**:
1. Not imported by any package
2. Not called by any function
3. Not stored in any struct field
4. Not registered as a callback/handler
5. Not implemented as an interface
6. Not used in tests AND not used in production

**NOT dead code**:
- Code with low usage count but serves extensibility
- Code that prevents import cycles
- Interfaces with single implementation (may enable testing/decoupling)
- Factory patterns that inject dependencies
- Code used only in specific modes (daemon, CLI, server)

### 3. Common Pitfalls to Avoid

❌ **DO NOT**:
- Rely solely on `grep "import"` to determine usage
- Assume low usage count means code is unnecessary
- Count lines as a proxy for "is this needed"
- Ignore initialization code in constructors
- Skip checking test files for usage
- Criticize patterns without understanding constraints
- Make assumptions about callback/event-driven usage

✅ **DO**:
- Search for actual function calls: `grep "FunctionName("`
- Search for struct field usage: `grep "\.fieldName"`
- Check initialization: `grep "fieldName:"`
- Read architectural comments explaining WHY
- Verify claims with multiple grep searches
- Consider "Why might this pattern exist?"
- Check daemon/server/CLI mode differences

### 4. Analysis Structure

For each potentially problematic package, provide:

#### A. Package Purpose
```
Package: internal/example/
Purpose: [from doc.go or package comment]
Files: [count] files, [count] lines (excluding tests)
```

#### B. Usage Verification
```
Exported Functions/Types:
- FunctionA: [count] usages
  - file1.go:line1
  - file2.go:line2
- FunctionB: [count] usages
  - file3.go:line3

Struct Fields:
- FieldX: initialized in [file:line], used in [file:line]

Callbacks/Events:
- HandlerY: registered in [file:line], called in [file:line]
```

#### C. Design Rationale
```
Why This Pattern Exists:
- [Reason 1 from code comments]
- [Reason 2 from architecture]
- [Constraint (e.g., import cycle prevention)]
```

#### D. Conclusion
```
Status: [ACTIVELY USED | DEAD CODE | EXTENSIBILITY POINT]
Recommendation: [KEEP | REFACTOR | REMOVE]
Confidence: [HIGH | MEDIUM | LOW]
```

### 5. Verification Checklist

Before finalizing any "dead code" claim, verify:

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
- [ ] Checked daemon/server mode differences

### 6. Search Commands to Use

**Finding Usage**:
```bash
# Search for function calls
grep -r "FunctionName(" --include="*.go" .

# Search for type usage
grep -r "TypeName{" --include="*.go" .

# Search for struct field usage
grep -r "\.fieldName" --include="*.go" .

# Search for interface implementations
grep -r "var _ InterfaceName" --include="*.go" .

# Search in specific package
grep -r "packagename\." internal/ --include="*.go"

# Count usages
grep -r "FunctionName(" --include="*.go" . | wc -l
```

**Understanding Design**:
```bash
# Read package docs
cat internal/package/doc.go

# Find architectural comments
grep -r "import cycle" --include="*.go" .
grep -r "TODO" --include="*.go" .
grep -r "HACK" --include="*.go" .

# Check initialization
grep -r "New.*(" internal/package/ --include="*.go"
```

### 7. Reporting Format

**Executive Summary**:
- Total packages analyzed: [count]
- Packages with dead code: [count]
- Packages with refactoring opportunities: [count]
- Total lines of removable code: [count]
- Confidence level: [HIGH | MEDIUM | LOW]

**For Each Finding**:
```markdown
### Package: internal/example/

**Claim**: [One sentence description]

**Evidence**:
- Exported: [list of exported symbols]
- Used: [count] times in production, [count] in tests
- Usage locations:
  - file1.go:line1 - [description of usage]
  - file2.go:line2 - [description of usage]

**Design Rationale** (if applicable):
- [Why this pattern exists]

**Recommendation**: [REMOVE | REFACTOR | KEEP]

**Confidence**: [HIGH | MEDIUM | LOW]
- HIGH: No usage found after thorough search
- MEDIUM: Low usage, unclear if intentional
- LOW: Unclear design intent, needs clarification
```

### 8. Self-Check Questions

Before submitting analysis, ask yourself:

1. Did I search for actual function calls, not just imports?
2. Did I check struct field usage, not just declarations?
3. Did I read the package documentation?
4. Did I understand why patterns exist before criticizing?
5. Did I check initialization in constructors?
6. Did I verify callback/event registration?
7. Did I provide file:line evidence for every claim?
8. Did I distinguish "unused" from "extensibility point"?
9. Did I check test files for usage?
10. Would I bet my reputation on each claim?

If answer to ANY question is "no", MORE VERIFICATION NEEDED.

---

## Example: Correct Analysis

### Package: internal/auth/

**Claim**: Package appears over-abstracted at first glance, but serves legitimate extensibility purpose.

**Evidence**:
- Exported: `Manager`, `CreateAuth()`, provider interfaces
- Production usage: 1 location
  - `internal/git/auth.go:13` - `auth.CreateAuth(authCfg)`
- Architecture: Manager → Registry → Provider interface → 4 concrete providers
- Files: 8 files (355 lines excluding tests)

**Design Rationale**:
- Registry pattern enables dynamic provider registration
- Interface enables testing with mock providers
- Follows Open/Closed Principle (add providers without modifying existing)
- Comment in `manager.go:35`: "Package-level instance for convenience"

**Usage Pattern**:
```go
// Single production call site
func createAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
    return auth.CreateAuth(authCfg)  // ✓ Used
}
```

**Recommendation**: KEEP
- Low usage count is intentional (single auth entry point)
- Pattern enables future auth types (OAuth, cert-based)
- Interface enables testing and mocking

**Confidence**: HIGH

---

## Example: Incorrect Analysis (What NOT to Do)

### ❌ WRONG Approach:

**Claim**: Package `internal/auth/` is over-abstracted, 8 files for 3 providers, can collapse to 1 file.

**Evidence**: Only used once in git package.

**Recommendation**: REMOVE registry, inline providers.

**Why This Is Wrong**:
- ❌ Didn't understand extensibility requirements
- ❌ Focused on line count, not architectural benefits
- ❌ Didn't read design comments
- ❌ Assumed low usage = unnecessary
- ❌ No consideration for future requirements

---

## Usage Instructions

1. **Paste this prompt** when requesting a reverse code review
2. **Reference specific methodology sections** if AI skips steps
3. **Demand evidence** if claims lack file:line references
4. **Challenge assumptions** if design rationale is missing
5. **Request re-verification** if analysis seems superficial

## Validation Script

After receiving analysis, validate with:

```bash
# For each "dead code" claim, verify yourself:
grep -r "ClaimedDeadFunction" --include="*.go" .

# Should return ZERO results for truly dead code
# If results found, analysis was WRONG
```

---

## Lessons from Failed Analysis (2026-01-04)

This prompt was created after a reverse code review that made 5 major factual errors:

1. **linkverify** - Claimed dead, was in production
2. **observability** - Claimed 80% unused, was 100% used
3. **auth** - Claimed over-abstracted, was extensible design
4. **services** - Claimed unnecessary, enabled decoupling
5. **eventstore** - Claimed test-only, was daemon feature

**Root Cause**: Only checked imports, not actual usage.

**Solution**: This prompt enforces thorough verification before making claims.

---

## Summary: The Golden Rule

> **Never claim code is dead/unnecessary without tracing actual usage through:**
> 1. Function calls
> 2. Struct fields
> 3. Initialization
> 4. Callbacks
> 5. Interface implementations
> 6. Test usage
> 7. Design rationale

**When in doubt**: Verify twice, claim once.
