#!/bin/bash
set -euo pipefail

echo "=== Testing Kaniko CI Approach ==="

# Check if Docker is available for testing
if ! command -v docker >/dev/null 2>&1; then
    echo "❌ Docker not available for Kaniko testing"
    exit 1
fi

# Test if we can run Kaniko container
echo "Testing Kaniko executor availability..."
if docker run --rm gcr.io/kaniko-project/executor:latest --help >/dev/null 2>&1; then
    echo "✅ Kaniko executor is accessible"
else
    echo "❌ Cannot access Kaniko executor"
    exit 1
fi

# Test basic Kaniko functionality with a simple Dockerfile
echo "Testing Kaniko build functionality..."
cat > /tmp/test-dockerfile << 'EOF'
FROM alpine:latest
RUN echo "Kaniko test successful" > /test.txt
CMD ["cat", "/test.txt"]
EOF

# Create a minimal context directory
mkdir -p /tmp/kaniko-test
cp /tmp/test-dockerfile /tmp/kaniko-test/Dockerfile

# Test Kaniko build (no-push mode)
if docker run --rm \
    -v /tmp/kaniko-test:/workspace \
    gcr.io/kaniko-project/executor:latest \
    --dockerfile=Dockerfile \
    --context=/workspace \
    --destination=test-kaniko-image:latest \
    --no-push >/dev/null 2>&1; then
    echo "✅ Kaniko build test successful"
else
    echo "❌ Kaniko build test failed"
    exit 1
fi

# Clean up
rm -rf /tmp/kaniko-test /tmp/test-dockerfile

echo "=== Kaniko CI approach validation completed successfully ==="
echo "Benefits of using Kaniko:"
echo "  ✅ No Docker daemon required"
echo "  ✅ Works in containerized CI environments"
echo "  ✅ Simpler permission model"
echo "  ✅ Built-in registry authentication"
echo "  ✅ Eliminates Docker-in-Docker complexity"