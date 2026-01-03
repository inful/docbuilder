---
title: "Transform Validation Reference (DEPRECATED)"
date: 2025-12-15
categories:
  - reference
tags:
  - validation
  - transforms
  - testing
  - deprecated
---

# Transform Pipeline Validation (DEPRECATED)

> **WARNING: DEPRECATED:** This document describes validation for the old registry-based transform system that was removed on December 16, 2025.
> 
> The new fixed transform pipeline uses explicit ordering and does not require dependency validation.
> See [ADR-003: Fixed Transform Pipeline](../adr/ADR-003-fixed-transform-pipeline.md) for current architecture.

---

## Historical Documentation

## Validation Features

### 1. Missing Dependency Detection

The validator checks that all transforms referenced in `MustRunAfter` and `MustRunBefore` declarations actually exist in the registry.

**Example Error:**
```
transform "my_enricher" depends on missing transform "nonexistent_builder" (MustRunAfter)
```

### 2. Circular Dependency Detection

The validator detects circular dependencies that would prevent the pipeline from establishing a valid execution order.

**Example Error:**
```
circular dependency detected: transform_a → transform_b → transform_a
```

### 3. Invalid Stage Validation

Transforms must declare one of the seven valid stages. Unknown stages are rejected.

**Valid stages:** `parse`, `build`, `enrich`, `merge`, `transform`, `finalize`, `serialize`

**Example Error:**
```
transform "custom" has invalid stage "preprocess"
```

### 4. Cross-Stage Dependency Warnings

The validator warns when a transform depends on another transform in a later stage, as this dependency cannot be enforced.

**Example Warning:**
```
transform "builder" (stage build) depends on "enricher" (stage enrich) which runs in a later stage - dependency may not be effective
```

### 5. Unused Transform Detection

Transforms that are not referenced by any other transform's dependencies receive a warning (informational only).

**Example Warning:**
```
transform "custom_filter" is not referenced by any other transform's dependencies - ensure it's intentionally standalone
```

## API Reference

### ValidatePipeline()

Performs comprehensive validation of the entire transform pipeline.

```go
result := transforms.ValidatePipeline()

if !result.Valid {
    for _, err := range result.Errors {
        log.Printf("ERROR: %s", err)
    }
}

for _, warn := range result.Warnings {
    log.Printf("WARNING: %s", warn)
}
```

**Returns:** `*ValidationResult` with `Valid` flag, `Errors` slice, and `Warnings` slice.

### GetPipelineInfo()

Returns a human-readable description of the current pipeline configuration.

```go
info, err := transforms.GetPipelineInfo()
if err != nil {
    log.Fatal(err)
}
fmt.Println(info)
```

**Output Example:**
```
Transform Pipeline Execution Order
===================================

Stage: parse
----------------------------------------
  • front_matter_parser

Stage: build
----------------------------------------
  • front_matter_builder_v2
    MustRunAfter: front_matter_parser

Stage: enrich
----------------------------------------
  • edit_link_injector_v2
    MustRunAfter: front_matter_builder_v2

...

Total transforms: 9
```

### PrintValidationResult()

Formats a validation result for display.

```go
result := transforms.ValidatePipeline()
output := transforms.PrintValidationResult(result)
fmt.Print(output)
```

**Output Example:**
```
Transform Pipeline Validation
==============================

✓ Pipeline is valid with no warnings
```

or with errors:

```
Transform Pipeline Validation
==============================

✗ Errors (2):
  1. transform "builder" depends on missing transform "parser_v3" (MustRunAfter)
  2. circular dependency detected: a → b → a

⚠ Warnings (1):
  1. transform "custom" is not referenced by any other transform's dependencies
```

### ValidatePipelineWithSuggestions()

Returns validation results along with helpful suggestions for fixing issues.

```go
result, suggestions := transforms.ValidatePipelineWithSuggestions()

if !result.Valid {
    fmt.Println("Validation failed!")
    for _, suggestion := range suggestions {
        fmt.Println(suggestion)
    }
}
```

### ListTransformNames()

Returns a sorted list of all registered transform names.

```go
names := transforms.ListTransformNames()
for _, name := range names {
    fmt.Println(name)
}
```

## Integration

### Automatic Validation

Validation runs automatically in `Generator.copyContentFiles()` before transform execution:

```go
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
    // Validate transform pipeline before execution
    if err := g.ValidateTransformPipeline(); err != nil {
        return fmt.Errorf("%w: %w", herrors.ErrContentTransformFailed, err)
    }
    
    // Build and execute transforms...
}
```

### Manual Validation

You can validate the pipeline programmatically:

```go
generator := hugo.NewGenerator(cfg, outputDir)
if err := generator.ValidateTransformPipeline(); err != nil {
    log.Fatalf("Pipeline validation failed: %v", err)
}
```

## Error Handling

Validation errors are **fatal** and will prevent the build from proceeding. This is intentional - a malformed pipeline would produce incorrect or incomplete output.

Warnings are **informational** and logged but do not prevent execution. They indicate potential issues that should be reviewed but may be intentional.

## Best Practices

1. **Test New Transforms:** Always run validation after adding new transforms to catch missing dependencies early.

2. **Review Warnings:** Don't ignore warnings - they often indicate configuration issues even if they're not fatal.

3. **Use Descriptive Names:** Clear transform names make validation messages easier to understand.

4. **Document Dependencies:** Comment why specific dependencies exist, especially for non-obvious orderings.

5. **Validate Before Commit:** Run validation as part of your pre-commit hooks or CI pipeline.

## Debugging Pipeline Issues

### View Current Pipeline

```go
info, _ := transforms.GetPipelineInfo()
fmt.Println(info)
```

### Check Specific Transform

```go
names := transforms.ListTransformNames()
if slices.Contains(names, "my_transform") {
    fmt.Println("Transform is registered")
}
```

### Validate Before Build

```go
result := transforms.ValidatePipeline()
if !result.Valid {
    // Fix issues before running build
}
```

## Testing

The validation system includes comprehensive test coverage:

- `TestValidatePipeline_Valid` - Valid pipeline passes
- `TestValidatePipeline_MissingDependency` - Detects missing dependencies
- `TestValidatePipeline_CircularDependency` - Detects cycles
- `TestValidatePipeline_InvalidStage` - Rejects invalid stages
- `TestValidatePipeline_CrossStageWarning` - Warns about cross-stage deps
- `TestValidatePipeline_EmptyRegistry` - Handles empty registry
- Plus 6 more tests for helper functions

Run validation tests:
```bash
go test ./internal/hugo/transforms -run TestValidate -v
```

## Performance

Validation is fast and runs only once at the start of content copying. The overhead is negligible (microseconds for typical pipelines with <20 transforms).

Results are not cached because the registry can change between builds, but within a single build, validation runs exactly once.
