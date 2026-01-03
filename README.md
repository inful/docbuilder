# DocBuilder

A Go CLI tool and daemon for aggregating documentation from multiple Git repositories into a unified Hugo static site.

## Features

- Clone and aggregate documentation from multiple Git repositories
- Multiple authentication methods: SSH keys, tokens, basic auth
- Incremental builds with change detection for faster CI pipelines
- Multi-version documentation from branches and tags
- Auto-discover repositories from forges (Forgejo, GitHub, GitLab)
- Hugo static site generation with Relearn theme
- Optional Hugo rendering for production-ready sites
- Daemon mode with live reload and webhook support
- Documentation linting with configurable rules
- Prometheus metrics and structured logging

## Installation

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/inful/docbuilder/releases):

```bash
# Linux (amd64)
wget https://github.com/inful/docbuilder/releases/latest/download/docbuilder_linux_amd64.tar.gz
tar -xzf docbuilder_linux_amd64.tar.gz
sudo mv docbuilder /usr/local/bin/

# macOS (arm64)
wget https://github.com/inful/docbuilder/releases/latest/download/docbuilder_darwin_arm64.tar.gz
tar -xzf docbuilder_darwin_arm64.tar.gz
sudo mv docbuilder /usr/local/bin/
```

### Docker

```bash
docker pull ghcr.io/inful/docbuilder:latest
```

### From Source

```bash
git clone https://github.com/inful/docbuilder.git
cd docbuilder
make build
```

## Quick Start

1. Initialize configuration:

```bash
docbuilder init -c config.yaml
```

2. Edit `config.yaml` and add your repositories:

```yaml
version: "2.0"

repositories:
  - url: https://github.com/org/repo.git
    name: my-docs
    branch: main
    paths: ["docs"]
    auth:
      type: token
      token: "${GITHUB_TOKEN}"

hugo:
  title: "My Documentation"
  base_url: "https://docs.example.com"

output:
  directory: "./site"
```

3. Build the site:

```bash
docbuilder build -v
```

## Configuration

DocBuilder uses YAML configuration with environment variable expansion. The application automatically loads `.env` and `.env.local` files.

### Basic Example

```yaml
version: "2.0"

repositories:
  - url: https://github.com/org/repo.git
    name: repo-name
    branch: main
    paths: ["docs"]
    auth:
      type: token
      token: "${GITHUB_TOKEN}"

hugo:
  title: "Documentation"
  base_url: "https://docs.example.com"

output:
  directory: "./site"
  clean: true
```

### Auto-Discovery Example

```yaml
version: "2.0"

forges:
  - name: "forgejo"
    type: "forgejo"
    api_url: "https://git.example.com/api/v1"
    auth:
      type: token
      token: "${FORGEJO_TOKEN}"

filtering:
  required_paths: ["docs"]

hugo:
  title: "Documentation"
  base_url: "https://docs.example.com"

output:
  directory: "./site"
```

### Authentication Methods

Token authentication (recommended):
```yaml
auth:
  type: token
  token: "${GIT_ACCESS_TOKEN}"
```

SSH key authentication:
```yaml
auth:
  type: ssh
  key_path: "~/.ssh/id_rsa"
```

Basic authentication:
```yaml
auth:
  type: basic
  username: "your-username"
  password: "${GIT_PASSWORD}"
```

## Commands

- `docbuilder build` - Build documentation site from configured repositories
- `docbuilder daemon` - Start daemon mode with live reload and webhooks
- `docbuilder preview` - Preview local docs with live reload
- `docbuilder generate` - Generate static site from local docs directory
- `docbuilder lint` - Lint documentation files for errors and style issues
- `docbuilder discover` - Discover documentation files without building
- `docbuilder init` - Initialize a new configuration file

Run `docbuilder <command> --help` for detailed options.

## Testing

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run golden tests
go test ./test/integration -run=TestGolden -v

# Update golden files
go test ./test/integration -run=TestGolden -update-golden
```

## Development

```bash
# Install dependencies
make deps

# Build
make build

# Run tests
make test

# Format code
make fmt

# Run linter
make lint
```

## Documentation

Complete documentation is available in the `docs/` directory:

- [Getting Started](docs/tutorials/getting-started.md) - Tutorial for first-time users
- [Configuration Reference](docs/reference/configuration.md) - Complete configuration options
- [CLI Reference](docs/reference/cli.md) - Command-line interface documentation
- [Architecture Overview](docs/explanation/architecture.md) - System architecture and design
- [How-To Guides](docs/how-to/) - Task-oriented guides for specific features

See [docs/README.md](docs/README.md) for complete documentation index.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

See [LICENSE](LICENSE) for license information.
