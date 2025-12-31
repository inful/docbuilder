// Package build provides the canonical build execution pipeline for DocBuilder.
//
// This package contains the core build service interface and implementation that
// coordinates the entire documentation generation workflow. All execution paths
// (CLI, daemon, tests) should route through BuildService.
//
// The package also defines sentinel errors for classifying high-level pipeline
// failures. These errors are used for retry semantics and should be wrapped
// with context at the call site.
package build
