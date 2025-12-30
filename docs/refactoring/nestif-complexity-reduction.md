---
title: "Nested Complexity Reduction Plan"
date: 2025-12-30
categories:
  - refactoring
  - code-quality
tags:
  - nestif
  - complexity
  - technical-debt
---

# Nested Complexity Reduction Plan

## Overview

The `nestif` linter has identified 42 instances of complex nested blocks across the codebase. This document outlines a systematic plan to reduce this complexity through refactoring.

## Complexity Distribution

- **Critical (≥15)**: 3 instances
- **High (10-14)**: 9 instances
- **Medium (7-9)**: 13 instances
- **Low (5-6)**: 17 instances

## Priority Areas for Refactoring

### Priority 1: Critical Complexity (≥15)

#### 1. `internal/hugo/modules.go:33` (complexity: 20)
**Function**: Module initialization logic
**Issue**: Deep nesting in module setup and Go module management
**Refactoring Strategy**:
- Extract module existence check into separate function
- Extract Go module initialization into dedicated method
- Use early returns to reduce nesting depth
- Consider builder pattern for module configuration

#### 2. `internal/daemon/http_server.go:337` (complexity: 19)
**Function**: HTTP server static file serving logic
**Issue**: Complex nested conditionals for path resolution and file serving
**Refactoring Strategy**:
- Extract path resolution logic into separate function
- Extract content type detection into dedicated method
- Use table-driven approach for MIME type mapping
- Consider strategy pattern for different file type handling

#### 3. `internal/hugo/classification.go:104` (complexity: 14)
**Function**: Error classification logic
**Issue**: Multiple nested conditions for error type detection
**Refactoring Strategy**:
- Use type switches instead of nested if statements
- Extract error pattern matching into separate functions
- Consider error classifier interface with multiple implementations

### Priority 2: High Complexity (10-14)

#### 4. `internal/daemon/build_queue.go:291` (complexity: 13)
**Function**: Event emission in build queue
**Issue**: Complex nested logic for event processing
**Refactoring Strategy**:
- Extract event building into separate function
- Use builder pattern for event construction
- Separate event emission logic from conditional checks

#### 5. `internal/daemon/status.go:140` (complexity: 13)
**Function**: Status projection logic
**Issue**: Deep nesting in status data aggregation
**Refactoring Strategy**:
- Extract projection building into dedicated method
- Use early returns for nil checks
- Consider separate status aggregator component

#### 6. `internal/hugo/indexes.go:284` (complexity: 13)
**Function**: Index file content parsing
**Issue**: Complex frontmatter parsing logic with nested conditions
**Refactoring Strategy**:
- Extract frontmatter parsing into separate parser struct
- Use state machine pattern for parsing logic
- Separate validation from parsing

#### 7. `internal/lint/fixer.go:192` (complexity: 13)
**Function**: Filename fix logic
**Issue**: Complex nested conditions for fix application
**Refactoring Strategy**:
- Extract fix validation into separate function
- Use command pattern for different fix types
- Separate dry-run logic from actual fix application

#### 8. `internal/auth/manager_test.go:106` (complexity: 12)
**Function**: Test assertion logic
**Issue**: Complex nested test validations
**Refactoring Strategy**:
- Extract assertion logic into helper functions
- Use table-driven test structure
- Create dedicated assertion helpers

### Priority 3: Medium Complexity (7-9)

Files with complexity 7-9 should be addressed after Priority 1 and 2:
- `internal/lint/fixer.go` (multiple instances at lines 640, 764, 925)
- `internal/lint/formatter.go:101`
- `internal/hugo/pipeline/transform_links.go` (lines 174, 249)
- `internal/hugo/renderer.go:77`
- `internal/server/handlers/webhook.go` (lines 107, 147)

**General Refactoring Strategies**:
- Extract nested loops into separate functions
- Use early returns to reduce nesting
- Apply guard clauses for precondition checks
- Extract complex boolean expressions into named functions

### Priority 4: Low Complexity (5-6)

17 instances with complexity 5-6. These should be addressed as part of regular maintenance:
- Focus on readability improvements
- Apply guard clauses
- Use early returns
- Extract small helper functions

## Common Refactoring Patterns

### Pattern 1: Extract Function
```go
// Before (complexity: 8)
if condition1 {
    if condition2 {
        if condition3 {
            // deep logic
        }
    }
}

// After (complexity: 3)
if condition1 {
    handleCondition1()
}

func handleCondition1() {
    if condition2 {
        handleCondition2()
    }
}
```

### Pattern 2: Early Returns (Guard Clauses)
```go
// Before (complexity: 7)
if condition1 {
    if condition2 {
        // logic
    } else {
        return err
    }
}

// After (complexity: 3)
if !condition1 {
    return nil
}
if !condition2 {
    return err
}
// logic
```

### Pattern 3: Replace Type Assertions with Type Switch
```go
// Before (complexity: 9)
if errors.As(err, &typeA) {
    if typeA.Field == value {
        // handle typeA
    }
} else if errors.As(err, &typeB) {
    // handle typeB
}

// After (complexity: 4)
switch e := err.(type) {
case *TypeA:
    return handleTypeA(e)
case *TypeB:
    return handleTypeB(e)
}
```

### Pattern 4: Strategy Pattern
```go
// Before (complexity: 10)
if fileType == "css" {
    if condition {
        // css specific logic
    }
} else if fileType == "js" {
    if condition {
        // js specific logic
    }
}

// After (complexity: 3)
handler := fileHandlers[fileType]
if handler != nil {
    return handler.Handle(file)
}
```

## Implementation Plan

### Phase 1: Critical Issues (Week 1)
- [ ] Refactor `internal/hugo/modules.go` (complexity: 20)
- [ ] Refactor `internal/daemon/http_server.go` (complexity: 19)
- [ ] Refactor `internal/hugo/classification.go` (complexity: 14)
- [ ] Run tests to ensure no regressions
- [ ] Update documentation if APIs change

### Phase 2: High Complexity Issues (Week 2)
- [ ] Refactor daemon build queue and status (complexity: 13)
- [ ] Refactor hugo indexes (complexity: 13)
- [ ] Refactor lint fixer core logic (complexity: 13)
- [ ] Refactor auth manager tests (complexity: 12)
- [ ] Run full test suite

### Phase 3: Medium Complexity Issues (Week 3)
- [ ] Refactor lint fixer link parsing (multiple instances)
- [ ] Refactor hugo pipeline transforms
- [ ] Refactor server webhook handlers
- [ ] Run integration tests

### Phase 4: Low Complexity Issues (Week 4)
- [ ] Address remaining instances (complexity 5-6)
- [ ] Final test suite run
- [ ] Update architectural documentation

## Success Criteria

1. **Zero Critical Issues**: No functions with complexity ≥ 15
2. **Minimal High Issues**: Fewer than 5 functions with complexity 10-14
3. **Test Coverage Maintained**: No reduction in test coverage
4. **No Regressions**: All existing tests pass
5. **Documentation Updated**: Reflect any architectural changes

## Tracking

Create tracking issues for each priority area:
- Issue #1: [Refactor] Reduce nested complexity in hugo modules (P1)
- Issue #2: [Refactor] Reduce nested complexity in daemon http server (P1)
- Issue #3: [Refactor] Reduce nested complexity in hugo error classification (P1)
- Issue #4: [Refactor] Reduce nested complexity in daemon build queue (P2)
- Issue #5: [Refactor] Reduce nested complexity in lint fixer (P2)

## Notes

- Each refactoring should be done in a separate PR
- Include before/after complexity metrics in PR description
- Maintain backward compatibility where possible
- Update unit tests to reflect new structure
- Consider adding integration tests for refactored areas

## References

- [Effective Go - Control Structures](https://go.dev/doc/effective_go#control-structures)
- [Go Code Review Comments - Indent Error Flow](https://github.com/golang/go/wiki/CodeReviewComments#indent-error-flow)
- [Martin Fowler - Refactoring Catalog](https://refactoring.com/catalog/)
