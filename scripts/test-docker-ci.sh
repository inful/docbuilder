#!/bin/bash
set -euo pipefail

echo "=== Testing Docker CI Setup (with installation) ==="

# Check what OS we're running on
if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "Detected OS: $ID $VERSION_ID"
else
    echo "Cannot detect OS distribution"
fi

# Function to install Docker on Ubuntu/Debian
install_docker() {
    echo "Installing Docker..."
    sudo apt-get update
    sudo apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg lsb-release
    
    # Detect OS distribution
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_ID=$ID
    else
        echo "Cannot detect OS distribution"
        exit 1
    fi
    
    # Set Docker repository based on OS
    case $OS_ID in
        ubuntu)
            DOCKER_REPO_URL="https://download.docker.com/linux/ubuntu"
            ;;
        debian)
            DOCKER_REPO_URL="https://download.docker.com/linux/debian"
            ;;
        *)
            echo "Unsupported OS: $OS_ID"
            exit 1
            ;;
    esac
    
    echo "Using Docker repository for $OS_ID: $DOCKER_REPO_URL"
    sudo mkdir -p /etc/apt/keyrings
    curl -fsSL $DOCKER_REPO_URL/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] $DOCKER_REPO_URL $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io
}

# Function to start Docker daemon
start_docker() {
    echo "Starting Docker daemon..."
    DOCKER_STARTED=false
    
    # Try systemctl if available and working
    if command -v systemctl >/dev/null 2>&1 && sudo systemctl is-system-running >/dev/null 2>&1; then
        echo "Trying systemctl..."
        if sudo systemctl start docker; then
            DOCKER_STARTED=true
            echo "Docker started via systemctl"
        fi
    fi
    
    # Try service if systemctl didn't work
    if [ "$DOCKER_STARTED" = "false" ] && command -v service >/dev/null 2>&1; then
        echo "Trying service..."
        if sudo service docker start; then
            DOCKER_STARTED=true
            echo "Docker started via service"
        fi
    fi
    
    # Try direct dockerd if other methods failed
    if [ "$DOCKER_STARTED" = "false" ] && command -v dockerd >/dev/null 2>&1; then
        echo "Trying direct dockerd..."
        sudo dockerd --host=unix:///var/run/docker.sock --log-level=error &
        DOCKER_STARTED=true
        echo "Docker daemon started directly"
    fi
    
    if [ "$DOCKER_STARTED" = "false" ]; then
        echo "Cannot find a way to start Docker daemon"
        exit 1
    fi
    
    # Wait for Docker to be ready
    echo "Waiting for Docker daemon to start..."
    for i in $(seq 1 30); do
        if docker info >/dev/null 2>&1; then
            echo "Docker daemon started successfully"
            return 0
        fi
        echo "Waiting for Docker... ($i/30)"
        sleep 1
    done
    
    # Final check
    if ! docker info >/dev/null 2>&1; then
        echo "Failed to start Docker daemon"
        exit 1
    fi
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