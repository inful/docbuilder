---
title: "DocBuilder Documentation"
date: 2025-12-15
categories:
  - documentation
tags:
  - overview
  - getting-started
---

# DocBuilder Documentation

Welcome to the DocBuilder documentation! This documentation follows the [Diátaxis](https://diataxis.fr/) framework, organizing content by user needs.

## Documentation Structure

### 📚 [Tutorials](tutorials/)
**Learning-oriented** - Step-by-step lessons to get you started.

- [Getting Started](tutorials/getting-started.md) - Your first DocBuilder project
- [Multi-Repository Setup](tutorials/multi-repo-setup.md) - Aggregate docs from multiple repos
- [Theme Customization](tutorials/theme-customization.md) - Customize your Hugo theme

**Best for:** First-time users, learning the basics

### 🛠️ [How-To Guides](how-to/)
**Task-oriented** - Practical guides for specific tasks.

- [Add Content Transforms](how-to/add-content-transforms.md) - Create custom markdown transformations
- [Add Theme Support](how-to/add-theme-support.md) - Integrate a new Hugo theme
- [Configure Forge Namespacing](how-to/configure-forge-namespacing.md) - Set up multi-forge projects
- [Customize Index Pages](how-to/customize-index-pages.md) - Tailor index page generation
- [Enable Hugo Render](how-to/enable-hugo-render.md) - Configure Hugo rendering
- [Prune Workspace Size](how-to/prune-workspace-size.md) - Optimize disk usage
- [Run Incremental Builds](how-to/run-incremental-builds.md) - Speed up builds

**Best for:** Users with specific goals, solving problems

### 📖 [Reference](reference/)
**Information-oriented** - Technical specifications and API documentation.

- [CLI Reference](reference/cli.md) - Command-line interface documentation
- [Configuration Reference](reference/configuration.md) - Complete config file specification
- [Index File Handling](reference/index-files.md) - How index files are processed and precedence rules
- [Build Report Reference](reference/report.md) - Build report format and fields

**Best for:** Looking up specific information, API details

### 💡 [Explanation](explanation/)
**Understanding-oriented** - Conceptual documentation and design rationale.

#### Architecture Documentation

- **[Architecture Documentation Index](explanation/README.md)** - Start here for architecture overview
- **[Comprehensive Architecture](explanation/comprehensive-architecture.md)** - Complete system design
- **[Architecture Diagrams](explanation/architecture-diagrams.md)** - Visual system representations
- **[Package Architecture Guide](explanation/package-architecture.md)** - Detailed package documentation
- **[Architecture Overview](explanation/architecture.md)** - Quick reference guide
- **[Namespacing Rationale](explanation/namespacing-rationale.md)** - Forge namespacing design
- **[Renderer Testing](explanation/renderer-testing.md)** - Hugo rendering tests

#### Architecture Decision Records (ADRs)

- [ADR-000: Uniform Error Handling](adr/ADR-000-uniform-error-handling.md)
- [ADR-001: Forge Integration Daemon](../plan/adr-001-forge-integration-daemon.md)

**Best for:** Understanding why things work the way they do, architectural decisions

## Quick Start Guide

### New Users

1. Start with [Getting Started Tutorial](tutorials/getting-started.md)
2. Review [CLI Reference](reference/cli.md) for commands
3. Check [Configuration Reference](reference/configuration.md) for options

### Developers/Contributors

1. Read [Architecture Documentation Index](explanation/README.md)
2. Study [Comprehensive Architecture](explanation/comprehensive-architecture.md)
3. Review [Package Architecture Guide](explanation/package-architecture.md)
4. Check [Contributing Guide](../CONTRIBUTING.md)

### Operations/DevOps

1. Review [Getting Started Tutorial](tutorials/getting-started.md)
2. Check [How-To: Run Incremental Builds](how-to/run-incremental-builds.md)
3. Review [Build Report Reference](reference/report.md)
4. Study operational considerations in [Comprehensive Architecture](explanation/comprehensive-architecture.md#operational-considerations)

## Documentation by Feature

### Basic Usage

- [Getting Started Tutorial](tutorials/getting-started.md)
- [CLI Reference](reference/cli.md)
- [Configuration Reference](reference/configuration.md)

### Multi-Repository Aggregation

- [Multi-Repository Setup Tutorial](tutorials/multi-repo-setup.md)
- [Configure Forge Namespacing](how-to/configure-forge-namespacing.md)
- [Namespacing Rationale](explanation/namespacing-rationale.md)

### Theme Integration

- [Theme Customization Tutorial](tutorials/theme-customization.md)
- [Add Theme Support](how-to/add-theme-support.md)
- [Relearn Theme Configuration](explanation/comprehensive-architecture.md#1-relearn-theme-configuration)

### Performance Optimization

- [Run Incremental Builds](how-to/run-incremental-builds.md)
- [Prune Workspace Size](how-to/prune-workspace-size.md)
- [Change Detection](explanation/comprehensive-architecture.md#3-change-detection)

### Customization

- [Add Content Transforms](how-to/add-content-transforms.md)
- [Customize Index Pages](how-to/customize-index-pages.md)
- [Index File Handling](reference/index-files.md)
- [Theme Customization Tutorial](tutorials/theme-customization.md)
- [Configuration Reference](reference/configuration.md)

### Advanced Topics

- [Comprehensive Architecture](explanation/comprehensive-architecture.md)
- [Package Architecture Guide](explanation/package-architecture.md)
- [Architecture Diagrams](explanation/architecture-diagrams.md)

## Additional Resources

### Project Documentation

- [README](../README.md) - Project overview and quick reference
- [CHANGELOG](../CHANGELOG.md) - Version history and changes
- [CONTRIBUTING](../CONTRIBUTING.md) - Contribution guidelines
- [LICENSE](../LICENSE) - Project license

### Architecture & Planning

- [Architecture Migration Plan (2025)](archive/architecture-migration-plan-2025.md) - Completed migration history
- [Plan Directory](../plan/) - Feature plans and ADRs
- [Docs Archive](archive/) - Historical documentation

### Examples

- [Examples Directory](../examples/) - Sample configurations and tools
- [Example Configs](../examples/configs/) - Ready-to-use configurations

## Documentation Principles

This documentation follows these principles:

1. **User-Centric** - Organized by what users want to achieve
2. **Progressive Disclosure** - Start simple, add complexity as needed
3. **Searchable** - Clear structure, consistent terminology
4. **Maintained** - Updated with code changes
5. **Tested** - Examples are verified to work

## Contributing to Documentation

We welcome documentation contributions! When contributing:

1. Follow the Diátaxis framework structure
2. Use clear, concise language
3. Include code examples where applicable
4. Test all commands and configurations
5. Update index files when adding new documents

See [Contributing Guide](../CONTRIBUTING.md) for details.

## Getting Help

- **Questions:** Open a GitHub Discussion
- **Issues:** Report bugs or feature requests via GitHub Issues
- **Architecture Questions:** Review [Architecture Documentation](explanation/README.md) first
- **Usage Help:** Start with [Tutorials](tutorials/) and [How-To Guides](how-to/)

## Documentation Status

**Last Major Update:** December 2025

**Coverage:**
- ✅ Getting Started Tutorial
- ✅ CLI Reference
- ✅ Configuration Reference
- ✅ Build Report Reference
- ✅ Comprehensive Architecture Documentation
- ✅ Package-Level Documentation
- ✅ Visual Architecture Diagrams
- ✅ How-To Guides
- ⏳ Additional tutorials (in progress)

**Feedback:** Documentation feedback is highly appreciated! Please open an issue if you find areas that need improvement.

---