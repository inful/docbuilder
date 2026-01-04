#!/bin/bash
set -e

echo "🔄 DocBuilder CI/CD Local Test Script"
echo "======================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test functions
test_go_build() {
    echo -e "\n${YELLOW}Testing Go build...${NC}"
    make clean
    make deps
    make build
    ./bin/docbuilder --version
    echo -e "${GREEN}✅ Go build test passed${NC}"
}

test_go_tests() {
    echo -e "\n${YELLOW}Running Go tests...${NC}"
    make test-coverage
    echo -e "${GREEN}✅ Go tests passed${NC}"
}

test_go_format() {
    echo -e "\n${YELLOW}Checking Go formatting...${NC}"
    if [ -n "$(gofmt -l .)" ]; then
        echo -e "${RED}❌ Code is not formatted. Run 'make fmt'${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Code formatting check passed${NC}"
}

test_docker_build() {
    echo -e "\n${YELLOW}Testing Docker build...${NC}"
    docker build -t docbuilder:ci-test .
    docker run --rm docbuilder:ci-test --version
    docker run --rm docbuilder:ci-test --help > /dev/null
    echo -e "${GREEN}✅ Docker build test passed${NC}"
}

test_docker_integration() {
    echo -e "\n${YELLOW}Testing Docker integration...${NC}"
    
    # Create test config
    cat > ci-test-config.yaml << EOF
hugo:
  title: "CI Test Docs"
  baseURL: "https://test.example.com"
  theme: relearn
  
repositories:
  - url: https://github.com/alecthomas/kong.git
    name: kong-docs
    branch: master
    paths: ["."]
EOF

    # Run integration test
    rm -rf ci-test-output
    mkdir -p ci-test-output
    
    docker run --rm \
        -v "$(pwd)/ci-test-config.yaml:/config.yaml:ro" \
        -v "$(pwd)/ci-test-output:/output" \
        docbuilder:ci-test \
        build -c /config.yaml -o /output --render-mode always -v
    
    # Check output
    if [ -d "ci-test-output/public" ]; then
        echo -e "${GREEN}✅ Docker integration test passed${NC}"
        echo "Generated files:"
        ls -la ci-test-output/public/ | head -5
    else
        echo -e "${RED}❌ Docker integration test failed${NC}"
        exit 1
    fi
    
    # Cleanup
    rm -f ci-test-config.yaml
    rm -rf ci-test-output
}

cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    docker rmi docbuilder:ci-test 2>/dev/null || true
    rm -f ci-test-config.yaml
    rm -rf ci-test-output
}

# Main execution
main() {
    echo "This script simulates the CI/CD pipeline locally"
    echo "It will run the same tests that run in Forgejo Actions"
    echo ""
    
    # Check dependencies
    if ! command -v go &> /dev/null; then
        echo -e "${RED}❌ Go is not installed${NC}"
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}❌ Docker is not installed${NC}"
        exit 1
    fi
    
    if ! command -v make &> /dev/null; then
        echo -e "${RED}❌ Make is not installed${NC}"
        exit 1
    fi
    
    # Run tests
    trap cleanup EXIT
    
    test_go_format
    test_go_build
    test_go_tests
    test_docker_build
    test_docker_integration
    
    echo -e "\n${GREEN}🎉 All CI/CD tests passed!${NC}"
    echo "Your changes should work well in the Forgejo CI pipeline."
}

# Run if executed directly
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    main "$@"
fi