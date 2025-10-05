#!/bin/bash
set -euo pipefail

echo "=== Testing Docker CI Setup (with installation) ==="

# Function to install Docker on Ubuntu/Debian
install_docker() {
    echo "Installing Docker..."
    sudo apt-get update
    sudo apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg lsb-release
    sudo mkdir -p /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io
}

# Function to start Docker daemon
start_docker() {
    echo "Starting Docker daemon..."
    sudo service docker start || sudo dockerd &
    # Wait for Docker to be ready
    timeout 30s sh -c 'until docker info >/dev/null 2>&1; do sleep 1; done'
}

# Test Docker availability
if command -v docker >/dev/null 2>&1; then
    echo "✅ Docker is available: $(docker --version)"
else
    echo "⚠️  Docker not found. Attempting installation..."
    if command -v apt-get >/dev/null 2>&1; then
        install_docker
    else
        echo "❌ Cannot install Docker on this system (not Ubuntu/Debian)"
        exit 1
    fi
fi

# Test Docker daemon
if docker info >/dev/null 2>&1; then
    echo "✅ Docker daemon is running"
else
    echo "⚠️  Docker daemon is not running. Attempting to start..."
    start_docker
fi

# Test basic Docker operations
echo "=== Testing Docker operations ==="

# Test simple container run
echo "Testing Docker run..."
if docker run --rm alpine:latest echo "Docker test successful"; then
    echo "✅ Docker run successful"
else
    echo "❌ Docker run failed"
    exit 1
fi

# Test simple image build
echo "Testing Docker build..."
cat > /tmp/test-dockerfile << 'EOF'
FROM alpine:latest
RUN echo "build test" > /test.txt
CMD ["cat", "/test.txt"]
EOF

if docker build -t test-ci-image -f /tmp/test-dockerfile /tmp >/dev/null 2>&1; then
    echo "✅ Docker build successful"
    
    # Test running the built image
    if docker run --rm test-ci-image | grep -q "build test"; then
        echo "✅ Docker built image run successful"
    else
        echo "❌ Docker built image run failed"
        exit 1
    fi
else
    echo "❌ Docker build failed"
    exit 1
fi

# Clean up
docker rmi test-ci-image >/dev/null 2>&1 || true
rm -f /tmp/test-dockerfile

echo "=== Docker CI setup test completed successfully ==="