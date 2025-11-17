# Docker Buildx Multi-Arch Local Build Fix

## Problem

When running the `.forgejo/workflows/buildx-multiarch.yml` workflow without registry credentials, the build fails with:

```text
ERROR: failed to build: docker exporter does not currently support exporting manifest lists
```

This happens because the workflow uses `--load` to load the built image into the local Docker daemon when credentials aren't available, but `--load` doesn't support multi-platform builds.

## Root Cause

Docker's `--load` option can only handle a single platform at a time. When you specify multiple platforms (like `linux/amd64,linux/arm64`) with `--load`, Docker Buildx fails because it can't load a manifest list (multi-platform image) into the local Docker daemon.

### The Dilemma

- **With `--push`**: Can build multiple platforms and push manifest list to registry ✅
- **With `--load`**: Can only build single platform and load to local Docker ⚠️
- **Without either**: Build succeeds but image isn't accessible ❌

## Solution

The workflow now **automatically detects the native platform** when building without credentials and only builds for that platform:

### Before (Broken)

```bash
# Always tried to build both platforms
BUILD_ARGS="--platform linux/amd64,linux/arm64"
if [ no_credentials ]; then
  BUILD_ARGS="$BUILD_ARGS --load"  # ❌ Fails with multi-platform
fi
```

### After (Fixed)

```bash
# Intelligently choose platform(s) based on credentials
if [ has_credentials ]; then
  PLATFORMS="linux/amd64,linux/arm64"  # Multi-arch
  BUILD_ARGS="--push"
else
  # Auto-detect native platform
  NATIVE_ARCH=$(uname -m)
  if [ "$NATIVE_ARCH" = "x86_64" ]; then
    PLATFORMS="linux/amd64"
  elif [ "$NATIVE_ARCH" = "aarch64" ] || [ "$NATIVE_ARCH" = "arm64" ]; then
    PLATFORMS="linux/arm64"
  else
    PLATFORMS="linux/amd64"  # fallback
  fi
  BUILD_ARGS="--load"  # ✅ Works with single platform
fi

BUILD_ARGS="--platform $PLATFORMS $BUILD_ARGS"
```

## Changes Made

### 1. Updated `.forgejo/workflows/buildx-multiarch.yml`

**Key Changes**:

- Added platform detection logic based on credential availability
- When no credentials: detect native arch with `uname -m` and build single platform
- When credentials available: build both `linux/amd64` and `linux/arm64`
- Updated log messages to clarify behavior

**Platform Detection**:


```bash
NATIVE_ARCH=$(uname -m)
if [ "$NATIVE_ARCH" = "x86_64" ]; then
  PLATFORMS="linux/amd64"
elif [ "$NATIVE_ARCH" = "aarch64" ] || [ "$NATIVE_ARCH" = "arm64" ]; then
  PLATFORMS="linux/arm64"
else
  PLATFORMS="linux/amd64"  # fallback for unknown architectures
fi
```

### 2. Updated Documentation

**File**: `docs/how-to/setup-multiarch-builds.md`

Added clarification about automatic behavior:

- With credentials → multi-arch build (amd64 + arm64) → push to registry
- Without credentials → native platform only → load locally
- Explains Docker limitation clearly

### 3. Updated Build Summary Output

The workflow now shows clearer messages:

**With Credentials**:

```text
✅ Multi-arch build and push completed successfully!
📋 Available at: git.home.luguber.info/org/repo:version
```

**Without Credentials**:

```text
⏭️  Skipping push (no registry credentials or push disabled)
📋 Built image locally for linux/amd64
💡 To build multi-arch, configure registry credentials and enable push
```

## Usage

### Local Testing (No Credentials)

The workflow will automatically:

1. Detect your native platform
2. Build only for that platform
3. Load the image into your local Docker daemon
4. You can use it immediately with `docker run`

```bash
# After workflow runs without credentials on amd64 machine:
docker images | grep docbuilder
# Shows: git.home.luguber.info/org/docbuilder:abc1234-full  (amd64 only)

# Use the image
docker run git.home.luguber.info/org/docbuilder:abc1234-full --version
```

### Production Builds (With Credentials)

Configure secrets in your Forgejo/GitHub repository:

1. Go to Settings → Secrets
2. Add `REGISTRY_USERNAME`
3. Add `REGISTRY_PASSWORD`

The workflow will then:

1. Build for both amd64 and arm64
2. Create manifest list
3. Push to registry
4. Images work on both architectures

## Benefits

✅ **Local development works** - No need for registry credentials to test builds  
✅ **Fast local builds** - Only builds for your platform  
✅ **Production ready** - Full multi-arch when credentials available  
✅ **Clear messaging** - Logs explain what's happening  
✅ **Automatic** - No manual platform selection needed

## Alternative Solutions Considered

### 1. Use Local Registry

```bash
# Start local registry
docker run -d -p 5000:5000 registry:2

# Push to local registry
--push --tag localhost:5000/image:tag
```

**Rejected**: Too complex for simple local testing

### 2. Build Twice (Once Per Platform)

```bash
docker buildx build --platform linux/amd64 --load ...
docker buildx build --platform linux/arm64 --load ...
```

**Rejected**: Doubles build time, requires platform matrix

### 3. Skip Build Without Credentials

```bash
if [ no_credentials ]; then
  echo "Skipping build - no credentials"
  exit 0
fi
```

**Rejected**: Makes local testing impossible

## Testing

To test the fix:

### Without Credentials (Local)

```bash
# Remove or disable credentials
# Run workflow
# Should build for native platform only and succeed
```

### With Credentials (Production)

```bash
# Configure REGISTRY_USERNAME and REGISTRY_PASSWORD
# Run workflow
# Should build for both platforms and push
```

## Related Issues

- Docker limitation: <https://github.com/docker/buildx/issues/59>
- Manifest lists in local daemon: Not supported
- Multi-platform local builds: Requires registry

## Migration Notes

If you were previously running this workflow locally:

**Before**: Would fail with manifest list error  
**After**: Builds successfully for your native platform

No breaking changes - behavior with credentials is identical to before.
