# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **BREAKING**: Consolidated error systems - migrated from `foundation.ClassifiedError` with 
  string-based `ErrorCode` to `errors.ClassifiedError` with type-safe `ErrorCategory` enum.
  All error construction now uses fluent builder API from `internal/foundation/errors` package.
  See [ADR-000](docs/adr/ADR-000-uniform-error-handling.md) for migration details.

### Fixed
- README.md files promoted to _index.md now preserve all transform pipeline changes
  (link rewrites, front matter patches, etc.). Previously, the index stage would
  re-read source files and bypass transforms. [ADR-002]
