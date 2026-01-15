.PHONY: build test clean install run init fmt lint

# Build the application
build:
	go build -o bin/docbuilder ./cmd/docbuilder

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf site/
	rm -f coverage.out coverage.html

# Install the binary
install: build
	cp bin/docbuilder $(GOPATH)/bin/

# Run the application (requires config.yaml). Tests use fixtures under test/testdata.
run: build
	./bin/docbuilder build

# Test discovery functionality
discover: build
	./bin/docbuilder discover -c test/testdata/config.test.yaml -v

# Initialize example configuration
init:
	./bin/docbuilder init

# Format code
fmt:
	golangci-lint fmt

# Lint code
lint:
	golangci-lint run --fix

# Development setup
dev-setup:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0

# Quick development cycle
dev: fmt build test