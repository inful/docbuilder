# How to Use Incremental Builds

Incremental builds allow DocBuilder to skip unchanged repositories, dramatically speeding up CI pipelines and daemon rebuilds.

## Enable Incremental Builds

Add to your configuration:

```yaml
build:
  enable_incremental: true
  cache_dir: .docbuilder-cache
  clone_strategy: auto  # Reuse clones when possible
```

## How It Works

### Build Signature

DocBuilder computes a signature from:
- Repository commit SHAs
- Repository content hashes (SHA256 of docs files)
- Hugo theme and version
- Configuration hash
- Transform list

### Cache Check Flow

1. **Compute current signature** from inputs
2. **Load previous manifest** from cache directory
3. **Compare signatures**:
   - Match → Skip build (instant exit)
   - Mismatch → Run full build
4. **Save new manifest** after successful build

### Cache Directory Structure

```
.docbuilder-cache/
  manifests/
    build-1702387200/
      manifest.json
    build-1702387800/
      manifest.json
```

Each manifest contains:
- Build ID and timestamp
- Combined signature hash
- Per-repository details (commit, hash, path)
- Theme and config fingerprints

## Use Cases

### CI Pipeline Optimization

```yaml
# .gitlab-ci.yml
build-docs:
  script:
    - docbuilder build -c config.yaml
  cache:
    paths:
      - .docbuilder-cache/
  artifacts:
    paths:
      - site/
    when: on_success
```

Benefits:
- Skips builds when only code changes (no doc changes)
- Preserves cache across pipeline runs
- Only generates artifacts when docs change

### Daemon Mode with Watch

```yaml
daemon:
  watch:
    enabled: true
    interval: 5m

build:
  enable_incremental: true
  cache_dir: /data/cache
```

Benefits:
- Periodic polls skip unchanged builds
- Reduces load on Git servers
- Minimizes Hugo rendering overhead

### Multi-Repository Projects

```yaml
repositories:
  - url: https://github.com/org/service-a.git
    name: service-a
  - url: https://github.com/org/service-b.git
    name: service-b
  - url: https://github.com/org/service-c.git
    name: service-c

build:
  enable_incremental: true
```

Future enhancements will enable per-repository caching:
- Only rebuild changed repositories
- Skip unchanged repositories entirely
- Merge cached and new content

## Performance Comparison

Without incremental builds:
```
Clone: 30s
Discovery: 5s
Hugo Generation: 10s
Total: 45s (every run)
```

With incremental builds (no changes):
```
Cache Check: 0.1s
Total: 0.1s (instant exit)
```

With incremental builds (1 repo changed):
```
Cache Check: 0.1s
Clone Changed: 5s
Discovery: 2s
Hugo Generation: 10s
Total: 17.1s (partial rebuild)
```

## Cache Management

### Clear Cache

```bash
rm -rf .docbuilder-cache/
```

### Inspect Cached Manifests

```bash
cat .docbuilder-cache/manifests/build-*/manifest.json | jq
```

### Cache Size

Manifests are small (typically <10KB each). Old manifests are not automatically cleaned up, but you can remove them periodically:

```bash
# Keep only last 10 builds
cd .docbuilder-cache/manifests
ls -t | tail -n +11 | xargs rm -rf
```

## Limitations

Current implementation:
- ✅ Build-level caching (all-or-nothing)
- ❌ Stage-level caching (planned)
- ❌ Per-repository caching (planned)

Future phases will enable:
- Skip individual unchanged repositories
- Incremental Hugo content merging
- Partial rebuild with cached base

## Troubleshooting

### Cache Not Working

Check that:
1. `enable_incremental: true` is set
2. Cache directory is writable
3. Cache directory persists between runs
4. No configuration changes between runs

### False Cache Hits

If builds are skipped incorrectly:
- Clear cache and rebuild
- Check that all inputs are included in signature
- Verify repository commit SHAs are accurate

### Cache Misses

If cache never hits:
- Check for volatile config fields (timestamps, random values)
- Verify cache directory is preserved
- Ensure consistent theme versions
