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
// Skip-evaluation is opt-in via BuildRequest.SkipState. The daemon passes
// its state manager; the CLI omits it.
package build
