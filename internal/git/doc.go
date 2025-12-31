// Package git provides a client for performing Git operations such as clone, update,
// and authentication handling for DocBuilder's documentation pipeline.
//
// This package handles Git operations including:
//   - Repository cloning with authentication (SSH, token, basic)
//   - Repository updates with divergence detection and resolution
//   - Remote head caching for optimization
//   - Retry logic for transient failures
//   - Typed errors for structured error handling
//
// The package provides functions for updating, synchronizing, and managing
// Git repositories with support for incremental updates and conflict resolution.
package git
