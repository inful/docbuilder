// Package workspace manages workspace directories for builds, supporting both
// ephemeral (timestamped) and persistent (fixed-path) modes.
//
// Ephemeral mode creates timestamped directories (e.g., docbuilder-20251214-122336)
// suitable for one-time builds, cleaning up completely after use.
//
// Persistent mode uses a fixed directory path (e.g., /data/repos/working) that
// persists across builds, enabling incremental updates and repository caching.
package workspace
