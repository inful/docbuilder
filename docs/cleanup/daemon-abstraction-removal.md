# Daemon Package Abstraction Removal Plan

**Date**: December 14, 2025  
**Status**: Ready for Implementation  
**Impact**: Medium - Removes ~300-400 lines of unnecessary abstraction  
**Risk**: Low - All changes are internal to daemon package

## Executive Summary

The `internal/daemon` package contains several over-engineered abstractions that provide no polymorphism benefit:
- Single-implementation interfaces used only once
- Orchestrator pattern wrapping simple operations
- Unnecessary indirection layers

This refactoring simplifies the codebase by inlining logic and removing unused abstractions.

## Files to Remove

### 1. `internal/daemon/live_reload_manager.go` (35 lines)
**Reason**: Unnecessary wrapper around `LiveReloadHub.Broadcast()`

**Current usage**:
```go
// Only used in post_persist_orchestrator.go
liveReloadManager: NewLiveReloadManager()
lrm.BroadcastUpdate(report, job)
```

**Migration**: Direct call to `LiveReloadHub`
```go
if job.TypedMeta != nil && job.TypedMeta.LiveReloadHub != nil {
    job.TypedMeta.LiveReloadHub.Broadcast(report.DocFilesHash)
}
```

### 2. `internal/daemon/post_persist_orchestrator.go` (120+ lines)
**Reason**: Single-use orchestrator that wraps three managers

**Current usage**: Only instantiated in `build_queue.go:113`
```go
orchestrator := NewPostPersistOrchestrator()
err = orchestrator.ExecutePostPersistStage(...)
```

**Migration**: Inline into `BuildQueue.processBuild()`

### 3. `internal/daemon/delta_manager.go` (150+ lines)
**Reason**: Interface with single implementation, no polymorphism needed

**Current usage**: Only used by `PostPersistOrchestrator`

**Migration**: Convert to standalone functions in `build_queue.go`

### 4. `internal/daemon/build_metrics_collector.go` (180+ lines)
**Reason**: Interface with single implementation, no polymorphism needed

**Current usage**: Only used by `PostPersistOrchestrator`

**Migration**: Convert to standalone functions or methods on `BuildQueue`

## Implementation Steps

### Phase 1: Inline PostPersistOrchestrator Logic

**File**: `internal/daemon/build_queue.go`

**Location**: In `processBuild()` method, after state persistence (line ~333)

**Current code**:
```go
// Execute post-persist stage
orchestrator := NewPostPersistOrchestrator()
err = orchestrator.ExecutePostPersistStage(PostPersistContext{
    BuildReport: report,
    DeltaPlan:   deltaPlan,
    Job:         job,
    StateAccess: bq.stateAccess,
})
```

**Replace with**:
```go
// Post-persistence operations
if err := bq.executePostPersist(ctx, report, deltaPlan, job); err != nil {
    slog.Error("Post-persist operations failed", "error", err)
    // Non-fatal - continue
}
```

**Add new method to BuildQueue**:
```go
// executePostPersist handles operations after state persistence
func (bq *BuildQueue) executePostPersist(
    ctx context.Context,
    report *hugo.BuildReport,
    deltaPlan *DeltaPlan,
    job *BuildJob,
) error {
    // 1. Attach delta metadata to report
    if deltaPlan != nil {
        attachDeltaMetadata(report, deltaPlan, job)
    }

    // 2. Recompute global doc hash
    deletionsDetected, err := recomputeGlobalDocHash(
        report,
        deltaPlan,
        bq.stateAccess,
    )
    if err != nil {
        slog.Warn("Failed to recompute global doc hash", "error", err)
    }

    // 3. Record deletions metric
    if deletionsDetected > 0 {
        EnsureTypedMeta(job)
        job.TypedMeta.DeletionFilesDetected = deletionsDetected
        bq.metrics.IncrementCounter("deleted_doc_files_detected")
        slog.Info("Detected deleted documentation files",
            "count", deletionsDetected,
            "job_id", job.ID)
    }

    // 4. Update repository metrics
    if err := updateRepositoryMetrics(report, bq.stateAccess); err != nil {
        slog.Warn("Failed to update repository metrics", "error", err)
    }

    // 5. Broadcast live reload
    if report.DocFilesHash != "" {
        EnsureTypedMeta(job)
        if job.TypedMeta.LiveReloadHub != nil {
            job.TypedMeta.LiveReloadHub.Broadcast(report.DocFilesHash)
        }
    }

    return nil
}
```

### Phase 2: Extract Delta Functions

**Create standalone functions** (can remain in `delta_manager.go` temporarily, then inline):

```go
// attachDeltaMetadata populates delta-related fields in the build report
func attachDeltaMetadata(report *hugo.BuildReport, deltaPlan *DeltaPlan, job *BuildJob) {
    if deltaPlan == nil {
        return
    }

    // Set delta decision
    report.DeltaDecision = string(deltaPlan.Decision)

    // Add repository reasons
    if len(deltaPlan.RepoReasons) > 0 {
        report.DeltaRepoReasons = make(map[string]string)
        for name, reason := range deltaPlan.RepoReasons {
            report.DeltaRepoReasons[name] = string(reason)
        }
    }

    // Set job metadata
    EnsureTypedMeta(job)
    job.TypedMeta.WasPartialBuild = (deltaPlan.Decision == DeltaDecisionPartial)
}

// recomputeGlobalDocHash rebuilds the global doc hash from unchanged + changed repos
func recomputeGlobalDocHash(
    report *hugo.BuildReport,
    deltaPlan *DeltaPlan,
    stateAccess DeltaStateAccess,
) (int, error) {
    // Implementation from DeltaManagerImpl.RecomputeGlobalDocHash
    // ... (keep existing logic)
}
```

### Phase 3: Extract Metrics Functions

**Create standalone functions**:

```go
// updateRepositoryMetrics updates per-repository document counts
func updateRepositoryMetrics(report *hugo.BuildReport, stateAccess DeltaStateAccess) error {
    if report == nil || stateAccess == nil {
        return nil
    }

    totalDocs, perRepoDocs, err := calculateDocumentCounts(report)
    if err != nil {
        return err
    }

    // Store total
    if err := stateAccess.StoreMetricInt("total_documents", totalDocs); err != nil {
        return fmt.Errorf("store total_documents: %w", err)
    }

    // Store per-repo counts
    for repoName, count := range perRepoDocs {
        key := fmt.Sprintf("repo:%s:documents", repoName)
        if err := stateAccess.StoreMetricInt(key, count); err != nil {
            slog.Warn("Failed to store repo document count",
                "repo", repoName,
                "error", err)
        }
    }

    slog.Debug("Updated repository metrics",
        "total_documents", totalDocs,
        "repositories", len(perRepoDocs))

    return nil
}

// calculateDocumentCounts computes total and per-repository document counts
func calculateDocumentCounts(report *hugo.BuildReport) (int, map[string]int, error) {
    // Implementation from BuildMetricsCollectorImpl.calculateDocumentCounts
    // ... (keep existing logic)
}
```

### Phase 4: Remove Files

Once the above changes are implemented and tested:

1. **Delete files**:
   ```bash
   rm internal/daemon/live_reload_manager.go
   rm internal/daemon/post_persist_orchestrator.go
   rm internal/daemon/delta_manager.go
   rm internal/daemon/build_metrics_collector.go
   ```

2. **Update build_queue.go**: Remove imports and type references

3. **Run tests**:
   ```bash
   go test ./internal/daemon/... -v
   go test ./... -v
   ```

## Testing Strategy

### Unit Tests

**Existing tests that need updates**:
- `internal/daemon/partial_global_hash_test.go` - Update to use new function signatures
- `internal/daemon/partial_global_hash_deletion_test.go` - Update to use new function signatures

**Test the new inlined logic**:
```go
func TestBuildQueue_ExecutePostPersist(t *testing.T) {
    // Test delta metadata attachment
    // Test hash recomputation
    // Test metrics updates
    // Test live reload broadcast
}
```

### Integration Tests

**Existing integration tests**:
- `internal/daemon/build_integration_test.go` - Should pass without changes
- `internal/daemon/discovery_state_integration_test.go` - Should pass without changes

**Verify**:
- Full build flow with delta detection
- Partial builds correctly update global hash
- Live reload notifications still work
- Metrics are correctly recorded

### Regression Testing

Run the full test suite:
```bash
go test ./... -v -count=1
```

Specific daemon tests:
```bash
go test ./internal/daemon/... -v -run TestPartialBuild
go test ./internal/daemon/... -v -run TestBuildQueue
go test ./internal/daemon/... -v -run TestLiveReload
```

## Migration Checklist

- [ ] **Phase 1**: Inline PostPersistOrchestrator
  - [ ] Add `executePostPersist` method to `BuildQueue`
  - [ ] Replace orchestrator call in `processBuild`
  - [ ] Run tests: `go test ./internal/daemon/build_queue_test.go`

- [ ] **Phase 2**: Extract delta functions
  - [ ] Create `attachDeltaMetadata` function
  - [ ] Create `recomputeGlobalDocHash` function
  - [ ] Update `executePostPersist` to use new functions
  - [ ] Run tests: `go test ./internal/daemon/partial_global_hash_test.go`

- [ ] **Phase 3**: Extract metrics functions
  - [ ] Create `updateRepositoryMetrics` function
  - [ ] Create `calculateDocumentCounts` function
  - [ ] Update `executePostPersist` to use new functions
  - [ ] Run tests: `go test ./internal/daemon/build_integration_test.go`

- [ ] **Phase 4**: Remove old files
  - [ ] Delete `live_reload_manager.go`
  - [ ] Delete `post_persist_orchestrator.go`
  - [ ] Delete `delta_manager.go`
  - [ ] Delete `build_metrics_collector.go`
  - [ ] Remove unused imports
  - [ ] Run full test suite: `go test ./...`

- [ ] **Phase 5**: Cleanup
  - [ ] Run `golangci-lint run ./internal/daemon/...`
  - [ ] Update documentation if needed
  - [ ] Commit with message: `refactor(daemon): remove unnecessary abstractions`

## Rollback Plan

If issues arise:

1. **Partial rollback**: Revert specific commits
   ```bash
   git revert <commit-hash>
   ```

2. **Full rollback**: Restore from backup branch
   ```bash
   git checkout main
   git branch -D refactor/daemon-cleanup
   ```

3. **Keep backup**: Before starting, create a backup branch
   ```bash
   git checkout -b backup/daemon-abstractions
   git checkout -b refactor/daemon-cleanup
   ```

## Expected Benefits

### Code Reduction
- **Lines removed**: ~300-400 lines of interface/wrapper code
- **Files removed**: 4 files
- **Complexity reduction**: Fewer indirection layers

### Performance
- **Negligible impact**: Removes virtual method calls (interface dispatch)
- **Memory**: Slightly less heap allocation from interface conversions

### Maintainability
- **Improved**: Direct, obvious code flow
- **Debugging**: Easier to trace execution
- **Understanding**: Less cognitive load for new contributors

### Code Quality Metrics

**Before**:
- Cyclomatic complexity: Higher (orchestrator pattern)
- Call depth: Deeper (3-4 levels)
- Testability: Requires mocking multiple interfaces

**After**:
- Cyclomatic complexity: Lower (direct calls)
- Call depth: Shallower (1-2 levels)
- Testability: Test actual implementation, fewer mocks

## Future Considerations

### Don't Over-Abstract

**Lesson learned**: Avoid creating interfaces until you have:
1. Multiple implementations
2. Clear polymorphism need
3. External dependency to mock

**Apply this to**:
- Other daemon components (review for similar patterns)
- New features (start simple, refactor when needed)

### When to Keep Interfaces

**Keep interfaces when**:
- Testing requires mocking external dependencies (HTTP, DB, filesystem)
- Multiple implementations exist or are planned
- Plugin/extension architecture
- Clear separation of concerns (e.g., `Builder` interface in daemon)

### Code Review Focus

In future PRs, watch for:
- Single-implementation interfaces
- Orchestrator patterns with no clear benefit
- Wrapper functions that just delegate
- Type conversion adapters between similar types

## References

- Go Proverbs: "The bigger the interface, the weaker the abstraction"
- Effective Go: "Interfaces in Go are implicit, create them when needed"
- Clean Code: "Don't repeat yourself" includes abstraction layers

## Approval

**Reviewed by**: _____________  
**Approved by**: _____________  
**Date**: _____________

---

**Next Steps**: Begin Phase 1 implementation on `refactor/daemon-cleanup` branch
