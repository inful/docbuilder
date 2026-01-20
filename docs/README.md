---
title: "DocBuilder Documentation"
date: 2025-12-15
categories:
  - documentation
tags:
  - overview
---

# DocBuilder Documentation

Technical documentation organized by the [Diátaxis](https://diataxis.fr/) framework.

## Structure

### 📚 [Tutorials](tutorials/)

Step-by-step learning guides:

- [Getting Started](tutorials/getting-started.md)

### 🛠️ [How-To Guides](how-to/)

Task-oriented guides:

- [Add Content Transforms](how-to/add-content-transforms.md)
- [Configure Forge Namespacing](how-to/configure-forge-namespacing.md)
- [Configure Webhooks](how-to/configure-webhooks.md)
- [Customize Index Pages](how-to/customize-index-pages.md)
- [Enable Hugo Render](how-to/enable-hugo-render.md)
- [Enable Multi-Version Docs](how-to/enable-multi-version-docs.md)
- [Enable Page Transitions](how-to/enable-page-transitions.md)
- [Prune Workspace Size](how-to/prune-workspace-size.md)
- [Run Incremental Builds](how-to/run-incremental-builds.md)
- [Setup Linting](how-to/setup-linting.md)
- [Use Relearn Theme](how-to/use-relearn-theme.md)
- [Write Cross-Document Links](how-to/write-cross-document-links.md)

### 📖 [Reference](reference/)

Technical specifications:

- [CLI Reference](reference/cli.md)
- [Configuration Reference](reference/configuration.md)
- [Content Transforms](reference/content-transforms.md)
- [Build Report Format](reference/report.md)
- [Index File Handling](reference/index-files.md)
- [Lint Rules](reference/lint-rules.md)

### 💡 [Explanation](explanation/)

Architecture and design rationale:

- [Architecture Documentation Index](explanation/README.md)
- [Comprehensive Architecture](explanation/comprehensive-architecture.md)
- [Architecture Diagrams](explanation/architecture-diagrams.md)
- [Package Architecture](explanation/package-architecture.md)
- [Namespacing Rationale](explanation/namespacing-rationale.md)
- [Skip Evaluation](explanation/skip-evaluation.md)
- [Renderer Testing](explanation/renderer-testing.md)
- [Webhook Isolation](explanation/webhook-documentation-isolation.md)

### Architecture Decision Records

- [ADR-000: Uniform Error Handling](adr/adr-000-uniform-error-handling.md)
- [ADR-001: Golden Testing Strategy](adr/adr-001-golden-testing-strategy.md)
- [ADR-002: In-Memory Content Pipeline](adr/adr-002-in-memory-content-pipeline.md)
- [ADR-003: Fixed Transform Pipeline](adr/adr-003-fixed-transform-pipeline.md)
- [ADR-004: Forge-Specific Markdown](adr/adr-004-forge-specific-markdown.md)
- [ADR-005: Documentation Linting](adr/adr-005-documentation-linting.md)
- [ADR-006: Drop Local Namespace](adr/adr-006-drop-local-namespace.md)
- [ADR-007: Merge Generate Into Build](adr/adr-007-merge-generate-into-build-command.md)
- [ADR-008: Staged Pipeline Architecture](adr/adr-008-staged-pipeline-architecture.md)
- [ADR-009: External Ingester Stage](adr/adr-009-external-ingester-stage.md)
- [ADR-010: Stable Document Identity via UID Aliases](adr/adr-010-stable-uid-aliases.md)
- [ADR-011: Set lastmod When Fingerprint Changes](adr/adr-011-lastmod-on-fingerprint-change.md)
- [ADR-012: Link-Safe File Normalization](adr/adr-012-link-safe-file-normalization.md)

## Quick Start

**New Users:**
1. [Getting Started Tutorial](tutorials/getting-started.md)
2. [CLI Reference](reference/cli.md)
3. [Configuration Reference](reference/configuration.md)

**Developers:**
1. [Architecture Documentation](explanation/README.md)
2. [Package Architecture](explanation/package-architecture.md)
3. [Contributing Guide](../CONTRIBUTING.md)

**Operations:**
1. [Run Incremental Builds](how-to/run-incremental-builds.md)
2. [Build Report Format](reference/report.md)
3. [Configure Webhooks](how-to/configure-webhooks.md)

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