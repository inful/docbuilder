# Hugo Renderer Testing Solution

## Problem Statement

**Original Issue**: Tests were failing in CI because they expected Hugo binary to be available, but CI environments don't have Hugo installed.

**Specific Failure**:
```
TestNoopRenderer: expected report.StaticRendered=true with NoopRenderer, got false
```

## Root Cause

The `stageRunHugo` function was checking `shouldRunHugo()` before checking for custom renderers. When `render_mode=always` and Hugo binary was missing, `shouldRunHugo()` returned false, causing early exit before the NoopRenderer could execute.

## Solution

### 1. Fixed Renderer Execution Order (`stage_run_hugo.go`)

Changed the execution order to prioritize custom renderers:

```go
func stageRunHugo(ctx context.Context, bs *BuildState) error {
    // 1. Check render_mode=never (early exit)
    if mode == config.RenderModeNever {
        return nil
    }
    
    // 2. Use custom renderer if set (NEW: moved before shouldRunHugo check)
    if bs.Generator.renderer != nil {
        if err := bs.Generator.renderer.Execute(root); err != nil {
            slog.Warn("Renderer execution failed", "error", err)
            return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", herrors.ErrHugoExecutionFailed, err))
        }
        bs.Report.StaticRendered = true
        return nil
    }
    
    // 3. Check if default Hugo binary should run
    if !shouldRunHugo(cfg) {
        return nil
    }
    
    // 4. Fallback to BinaryRenderer
    if err := bs.Generator.runHugoBuild(); err != nil {
        return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", build.ErrHugo, err))
    }
    bs.Report.StaticRendered = true
    return nil
}
```

**Key Change**: Custom renderer check now happens before `shouldRunHugo()`, ensuring injected renderers (like NoopRenderer) work even when Hugo binary is missing.

### 2. Added NoopRenderer to Report Persistence Tests

Updated tests that were failing due to Hugo binary requirement:

```go
// Before
gen := NewGenerator(cfg, out)

// After
cfg.Build.RenderMode = "always"
gen := NewGenerator(cfg, out).WithRenderer(&NoopRenderer{})
```

**Files Updated**:
- `internal/hugo/report_persist_test.go` - 2 tests updated

### 3. Created Comprehensive Integration Tests

Added `renderer_integration_test.go` with tests for both NoopRenderer and BinaryRenderer:

**Unit Tests (No Hugo Required)**:
- `TestNoopRenderer` (existing) - Verifies NoopRenderer marks site as rendered
- `TestRenderMode_Never_SkipsRendering` - Verifies no rendering with mode=never
- `TestRenderMode_Always_WithNoopRenderer` - Verifies custom renderer precedence
- `TestRenderMode_Auto_WithoutEnvVars` - Verifies auto mode behavior

**Integration Tests (Hugo Optional)**:
- `TestBinaryRenderer_WhenHugoAvailable` - Tests real Hugo execution (skips if unavailable)
- `TestBinaryRenderer_MissingHugoBinary` - Tests error handling
- `TestRendererPrecedence` - Comprehensive test matrix

**Smart Skipping**:
```go
if _, err := exec.LookPath("hugo"); err != nil {
    t.Skip("Hugo binary not found; skipping integration test")
}
```

## Testing Strategy

### CI Environment (No Hugo)
- All tests use `NoopRenderer`
- Fast execution
- No external dependencies
- Tests logic without requiring Hugo

### Local Development (Hugo Available)  
- Integration tests auto-detect Hugo
- Run real Hugo rendering
- Verify end-to-end functionality
- Gracefully handle Hugo failures

## Benefits

1. **CI Compatibility**: Tests pass in CI without Hugo binary
2. **Local Integration**: Developers with Hugo get full integration testing
3. **Clear Separation**: Unit tests vs integration tests are distinct
4. **Maintainability**: Easy to add new tests following established patterns
5. **Documentation**: Comprehensive testing guide for contributors

## Test Results

### Before Fix
```
FAIL: TestNoopRenderer (0.00s)
    renderer_test.go:37: expected report.StaticRendered=true with NoopRenderer, got false
FAIL: TestReportPersistence_Success (0.33s)
    report_persist_test.go:33: expected outcome=success got warning
```

### After Fix
```
PASS: TestNoopRenderer (0.00s)
PASS: TestReportPersistence_Success (0.01s)
PASS: TestReportPersistence_FailureDoesNotOverwrite (0.00s)
PASS: TestBinaryRenderer_WhenHugoAvailable (0.26s)
PASS: TestRenderMode_Never_SkipsRendering (0.00s)
PASS: TestRenderMode_Always_WithNoopRenderer (0.00s)
PASS: TestRenderMode_Auto_WithoutEnvVars (0.24s)
PASS: TestRendererPrecedence (0.24s)
    ✓ All 8 renderer tests passing
```

## Files Changed

1. **`internal/hugo/stage_run_hugo.go`**
   - Reordered renderer execution logic
   - Custom renderers now checked before `shouldRunHugo()`

2. **`internal/hugo/report_persist_test.go`**
   - Added NoopRenderer to 2 tests
   - Set `render_mode=always` explicitly

3. **`internal/hugo/renderer_integration_test.go`** (NEW)
   - 8 comprehensive tests
   - Unit tests for NoopRenderer
   - Integration tests for BinaryRenderer
   - Smart Hugo detection and skipping

4. **`docs/explanation/renderer-testing.md`** (NEW)
   - Complete testing strategy documentation
   - Examples and best practices
   - Troubleshooting guide

## Future Considerations

### Adding New Tests

**For CI-friendly unit tests**:
```go
g := NewGenerator(cfg, dir).WithRenderer(&NoopRenderer{})
```

**For Hugo integration tests**:
```go
if _, err := exec.LookPath("hugo"); err != nil {
    t.Skip("Hugo not available")
}
g := NewGenerator(cfg, dir) // Uses BinaryRenderer
```

### Custom Renderers

The pattern supports future custom renderers:
- Remote rendering services
- In-process Hugo library (if available)
- Mock renderers for specific test scenarios

## Conclusion

This solution provides a robust testing framework that:
- ✅ Works in CI without Hugo
- ✅ Provides integration testing when Hugo is available
- ✅ Documents the testing approach clearly
- ✅ Makes it easy to add new tests
- ✅ Maintains backward compatibility

The key insight: **Custom renderers should take precedence over environment checks**, allowing tests to inject mock renderers without worrying about system state.
