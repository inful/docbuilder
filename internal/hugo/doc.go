// Package hugo implements the Hugo site generation pipeline for DocBuilder.
//
// # Architecture
//
// The generator composes a series of high‑level build "stages" (prepare_output,
// generate_config, layouts, copy_content, indexes, run_hugo, post_process).
// Each stage operates on a shared mutable BuildState that carries configuration,
// discovered documentation files, and timing instrumentation. Stage execution
// order is strictly defined in generator.go and measured in stages.go; timings
// are exported through BuildReport.StageDurations for observability.
//
// Within the copy_content stage a finer grained transformation pipeline runs
// per markdown file. The pipeline currently includes:
//  1. FrontMatterParser             – parse any existing front matter
//  2. FrontMatterBuilder            – synthesize canonical front matter
//  3. EditLinkInjector              – inject per‑page repository edit URLs
//  4. RelativeLinkRewriter          – rewrite intra‑repo relative links
//  5. FinalFrontMatterSerializer    – serialize the merged front matter
//
// This two‑tier design (coarse build stages + per‑file transformers) keeps
// responsibilities isolated, makes incremental refactors safer, and allows
// future stages/transformers (search indexing, syntax highlighting metadata,
// linting, etc.) to be slotted in with minimal coupling.
//
// The package purposefully avoids global state; all configuration flows in via
// config.Config provided to NewGenerator. Long‑term enhancements (cancellation,
// tracing hooks, structured stage error types) can build upon the existing
// Stage + BuildReport abstractions without altering public call semantics.
package hugo
