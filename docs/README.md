# DocBuilder Documentation

Documentation organized by the [Diátaxis](https://diataxis.fr/) framework to help you find what you need.

## Documentation Structure

### Tutorials

Learning-oriented guides for getting started.

- [Getting Started](tutorials/getting-started.md) - Your first DocBuilder project

### How-To Guides

Task-oriented guides for specific objectives.

- [Add Content Transforms](how-to/add-content-transforms.md) - Create custom markdown transformations
- [Configure Forge Namespacing](how-to/configure-forge-namespacing.md) - Set up multi-forge projects
- [Customize Index Pages](how-to/customize-index-pages.md) - Tailor index page generation
- [Enable Hugo Render](how-to/enable-hugo-render.md) - Configure Hugo rendering
- [Enable Multi-Version Docs](how-to/enable-multi-version-docs.md) - Build from multiple branches/tags
- [Enable Page Transitions](how-to/enable-page-transitions.md) - Add View Transitions API
- [Prune Workspace Size](how-to/prune-workspace-size.md) - Optimize disk usage
- [Run Incremental Builds](how-to/run-incremental-builds.md) - Speed up builds
- [Setup Linting](how-to/setup-linting.md) - Configure documentation linting
- [Use Relearn Theme](how-to/use-relearn-theme.md) - Configure Relearn theme
- [Write Cross-Document Links](how-to/write-cross-document-links.md) - Link between documents

### Reference

Technical specifications and API documentation.

- [CLI Reference](reference/cli.md) - Command-line interface
- [Configuration Reference](reference/configuration.md) - Complete config specification
- [Content Transforms](reference/content-transforms.md) - Available transformations
- [Index File Handling](reference/index-files.md) - Index file processing and precedence
- [Build Report Reference](reference/report.md) - Build report format and fields
- [Lint Rules](reference/lint-rules.md) - Documentation linting rules

### Explanation

Conceptual documentation and design rationale.

- [Architecture Overview](explanation/architecture.md) - System architecture summary
- [Comprehensive Architecture](explanation/comprehensive-architecture.md) - Detailed system design
- [Package Architecture](explanation/package-architecture.md) - Package-level documentation
- [Namespacing Rationale](explanation/namespacing-rationale.md) - Forge namespacing design

See [Architecture Documentation](explanation/README.md) for complete architecture guides.

### Architecture Decision Records

- [ADR-000: Uniform Error Handling](adr/adr-000-uniform-error-handling.md)
- [ADR-001: Golden Testing Strategy](adr/adr-001-golden-testing-strategy.md)
- [ADR-002: In-Memory Content Pipeline](adr/adr-002-in-memory-content-pipeline.md)
- [ADR-003: Fixed Transform Pipeline](adr/adr-003-fixed-transform-pipeline.md)
- [ADR-004: Forge-Specific Markdown](adr/adr-004-forge-specific-markdown.md)
- [ADR-005: Documentation Linting](adr/adr-005-documentation-linting.md)
- [ADR-006: Drop Local Namespace](adr/adr-006-drop-local-namespace.md)

## Quick Navigation

### New Users

1. Start with [Getting Started](tutorials/getting-started.md)
2. Review [CLI Reference](reference/cli.md)
3. Check [Configuration Reference](reference/configuration.md)

### Developers/Contributors

1. Read [Architecture Overview](explanation/architecture.md)
2. Review [Package Architecture](explanation/package-architecture.md)
3. Check [Contributing Guide](../CONTRIBUTING.md)

### Operations/DevOps

1. Review [Getting Started](tutorials/getting-started.md)
2. Check [Run Incremental Builds](how-to/run-incremental-builds.md)
3. Review [Build Report Reference](reference/report.md)

## Contributing

When contributing documentation:

1. Follow the Diátaxis framework structure
2. Use clear, concise language
3. Include tested code examples
4. Update index files when adding documents

See [Contributing Guide](../CONTRIBUTING.md) for details.

## Getting Help

- Questions: Open a GitHub Discussion
- Issues: Report bugs or feature requests via GitHub Issues
- Architecture: Review [Architecture Documentation](explanation/README.md)
- Usage: Start with [Tutorials](tutorials/) and [How-To Guides](how-to/)
