# docbuilder

A Go utility for creating documentation sites from multiple Git repositories using Hugo.

## Features

- Clone documentation from multiple Git repositories
- Support for various authentication methods (SSH, tokens, basic auth)
- Generate Hugo-compatible static sites
- Environment variable support with `.env` files
- Incremental builds for faster updates

## Quick Start

1. **Build the application**:
   ```bash
   make build
   ```

2. **Initialize configuration**:
   ```bash
   ./bin/docbuilder init
   ```

3. **Set up environment variables** (optional):
   ```bash
   cp .env.example .env
   # Edit .env with your credentials
   ```

4. **Build documentation site**:
   ```bash
   ./bin/docbuilder build -v
   ```

## Configuration

### Environment Variables

The application automatically loads environment variables from `.env` and `.env.local` files. Example:

```bash
# .env
GIT_ACCESS_TOKEN=your_git_access_token_here
```

### Configuration File

Example `config.yaml`:

```yaml
repositories:
  - url: https://git.example.com/owner/repo.git
    name: my-docs
    branch: main
    paths:
      - docs
    auth:
      type: token
      token: "${GIT_ACCESS_TOKEN}"

hugo:
  title: "My Documentation Site"
  description: "Aggregated documentation"
  base_url: "https://docs.example.com"

output:
  directory: "./site"
  clean: true
```

### Authentication Methods

1. **Token Authentication** (recommended):
   ```yaml
   auth:
     type: token
     token: "${GIT_ACCESS_TOKEN}"
   ```

2. **SSH Key Authentication**:
   ```yaml
   auth:
     type: ssh
     key_path: "~/.ssh/id_rsa"
   ```

3. **Basic Authentication**:
   ```yaml
   auth:
     type: basic
     username: "your-username"
     password: "${GIT_PASSWORD}"
   ```

## Commands

- `docbuilder build` - Build documentation site from configured repositories
- `docbuilder init` - Initialize a new configuration file
- `docbuilder build --incremental` - Update existing repositories instead of fresh clone

## Development

```bash
# Install dependencies
make deps

# Run tests
make test

# Format code
make fmt

# Development cycle
make dev
```