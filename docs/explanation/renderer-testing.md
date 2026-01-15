---
uid: 1575ffc4-7bf0-46df-a8b2-904e93f95031
aliases:
  - /_uid/1575ffc4-7bf0-46df-a8b2-904e93f95031/
title: "Renderer Testing"
date: 2025-12-15
categories:
  - explanation
tags:
  - testing
  - renderer
fingerprint: 1193625675d0289d92d95ee2323275dd943fcdb57987b6a5d6516745758ebc82
---

# Hugo Renderer Testing Strategy

## Overview

The Hugo renderer system uses a dual-path testing approach to ensure both the NoopRenderer (for CI/fast tests) and BinaryRenderer (for integration tests) work correctly.

## Test Files

### `renderer_test.go`
**Purpose**: Fast unit tests that don't require Hugo binary

**Key Tests**:
- `TestNoopRenderer`: Verifies NoopRenderer marks site as rendered without invoking Hugo
  - Uses `WithRenderer(&NoopRenderer{})` to inject test renderer
  - Sets `render_mode=always` to ensure rendering is attempted
  - Verifies `StaticRendered=true` even when Hugo binary is missing

### `renderer_integration_test.go`  
**Purpose**: Integration tests that verify actual Hugo execution when available

**Key Tests**:

1. **`TestBinaryRenderer_WhenHugoAvailable`**
   - Skips if Hugo not in PATH
   - Verifies BinaryRenderer invokes real Hugo binary
   - Gracefully handles Hugo failures (e.g., missing theme dependencies)
   - Checks for `public/` directory creation

2. **`TestBinaryRenderer_MissingHugoBinary`**
   - Verifies proper error handling when Hugo unavailable
   - Tests the BinaryRenderer error path directly

3. **`TestRenderMode_Never_SkipsRendering`**
   - Verifies `render_mode=never` prevents all rendering
   - Ensures no `public/` directory is created

4. **`TestRenderMode_Always_WithNoopRenderer`**
   - Verifies custom renderer takes precedence
   - NoopRenderer should run even with `render_mode=always`

5. **`TestRenderMode_Auto_WithoutEnvVars`**
   - Verifies `render_mode=auto` behavior
   - Tests legacy env var handling

6. **`TestRendererPrecedence`**
   - Comprehensive test matrix documenting renderer selection priority
   - Tests all combinations of render modes and renderer types

## Testing Strategy

### CI Environment (No Hugo Binary)

All tests use `NoopRenderer` to avoid requiring Hugo installation:

```go
cfg.Build.RenderMode = "always"
g := NewGenerator(cfg, dir).WithRenderer(&NoopRenderer{})
```

**Benefits**:
- Fast test execution
- No external dependencies
- Tests the rendering pipeline logic
- Verifies `StaticRendered` tracking

### Local Development (Hugo Available)

Integration tests automatically detect Hugo and run real rendering:

```go
if _, err := exec.LookPath("hugo"); err != nil {
    t.Skip("Hugo binary not found; skipping integration test")
}
// Test runs with real Hugo binary
```

**Benefits**:
- Verifies end-to-end Hugo integration
- Catches Hugo-specific issues
- Tests actual static site generation

## Renderer Selection Priority

The actual priority (from `stage_run_hugo.go`):

1. **`render_mode=never`** → Skip all rendering (return early)
2. **Custom renderer set** → Use custom renderer (e.g., NoopRenderer)
3. **`shouldRunHugo()` check** → Evaluate render mode and Hugo availability
4. **Fallback** → Use BinaryRenderer with Hugo binary

```go
func stageRunHugo(ctx context.Context, bs *BuildState) error {
    // 1. Check render_mode=never
    if mode == config.RenderModeNever {
        return nil
    }
    
    // 2. Use custom renderer if set
    if bs.Generator.renderer != nil {
        // Execute custom renderer (e.g., NoopRenderer)
        // Sets StaticRendered=true on success
    }
    
    // 3. Check if default Hugo binary should run
    if !shouldRunHugo(cfg) {
        return nil
    }
    
    // 4. Fallback to BinaryRenderer
    bs.Generator.runHugoBuild()
}
```

## Adding New Renderer Tests

### For Unit Tests (No Hugo Required)

Use NoopRenderer and test the logic:

```go
func TestMyFeature(t *testing.T) {
    cfg := &config.Config{}
    cfg.Build.RenderMode = "always"
    g := NewGenerator(cfg, t.TempDir()).WithRenderer(&NoopRenderer{})
    
    // Test your feature
}
```

### For Integration Tests (Hugo Required)

Skip when Hugo unavailable:

```go
func TestMyHugoFeature(t *testing.T) {
    if _, err := exec.LookPath("hugo"); err != nil {
        t.Skip("Hugo not available")
    }
    
    g := NewGenerator(cfg, t.TempDir()) // Uses BinaryRenderer
    // Test with real Hugo
}
```

## Common Patterns

### Test Renderer Execution Path

```go
// Verify NoopRenderer was used
g := NewGenerator(cfg, dir).WithRenderer(&NoopRenderer{})
report, _ := g.GenerateSite(files)
if !report.StaticRendered {
    t.Error("NoopRenderer should set StaticRendered=true")
}
```

### Test Hugo Failure Handling

```go
// Hugo may fail but shouldn't crash the build
g := NewGenerator(cfg, dir) // BinaryRenderer
report, err := g.GenerateSite(files)
// err should be nil (warnings don't return errors)
// report.StaticRendered may be false if Hugo failed
```

### Test Render Mode Behavior

```go
// Test each mode
modes := []config.RenderMode{
    config.RenderModeNever,  // No rendering
    config.RenderModeAlways, // Always attempt
    config.RenderModeAuto,   // Conditional
}
```

## Debugging Test Failures

### "Hugo binary not found" in CI
✅ **Expected** - Use NoopRenderer for CI tests

### "StaticRendered=false" with BinaryRenderer  
✅ **Expected** - Hugo may fail without proper theme setup  
Check logs for "Renderer execution failed"

### "public/ directory exists but StaticRendered=false"
✅ **Expected** - Hugo creates public/ before failing  
This is normal for partial renders

### Custom renderer not being used
❌ **Problem** - Check that `WithRenderer()` is called  
Verify render_mode is not "never"

## Best Practices

1. **Always use NoopRenderer in CI** - Don't depend on Hugo being installed
2. **Skip integration tests gracefully** - Use `t.Skip()` when Hugo unavailable  
3. **Test the interface, not implementation** - Focus on `StaticRendered` and `public/` dir
4. **Handle Hugo failures gracefully** - Real Hugo may fail in tests, that's OK
5. **Document test expectations** - Use clear test names and comments

## Example: Complete Test

```go
func TestExampleFeature(t *testing.T) {
    // Setup
    cfg := &config.Config{}
    cfg.Hugo.Theme = "relearn"
    cfg.Build.RenderMode = "always"
    
    dir := t.TempDir()
    
    // Use NoopRenderer for fast CI-friendly tests
    g := NewGenerator(cfg, dir).WithRenderer(&NoopRenderer{})
    
    doc := docs.DocFile{
        Repository: "test",
        Name:       "test",
        RelativePath: "test.md",
        DocsBase:   "docs",
        Extension:  ".md",
        Content:    []byte("# Test\n"),
    }
    
    // Execute
    report, err := g.GenerateSiteWithReportContext(context.Background(), []docs.DocFile{doc})
    
    // Verify
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if !report.StaticRendered {
        t.Error("expected StaticRendered=true with NoopRenderer")
    }
    
    // NoopRenderer doesn't create public/, that's expected
    publicDir := filepath.Join(dir, "public")
    if _, err := os.Stat(publicDir); err == nil {
        t.Error("NoopRenderer shouldn't create public/ directory")
    }
}
```

## Related Documentation

- [Renderer Architecture](../explanation/architecture.md#renderer-system)
- [CI/CD Setup](../ci-cd-setup.md)
