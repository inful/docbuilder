// Package build provides the canonical build execution pipeline for DocBuilder.
//
// This package contains the core build service interface and implementation that
// coordinates the entire documentation generation workflow. All execution paths
// (CLI, daemon, tests) should route through BuildService.
//
// BuildService exposes two entry points:
//
//   - Run: full pipeline — workspace, clone, discover, generate, render.
//     Use for builds where the caller has a list of repositories.
//   - RunDirect: partial pipeline — generate, render. Use when the caller
//     has already produced the doc-file list (e.g. preview mode, or
//     `docbuilder build -d` against a local directory).
//
// A build is skipped by the pipeline itself: after the repositories are
// cloned, the pipeline compares their commits with the ones the last build
// used, and skips the rest of the work when nothing moved.
package build
