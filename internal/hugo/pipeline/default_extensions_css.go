package pipeline

// defaultCustomCSS is the built-in override written to
// assets/css/extensions.css in the Hugo site source when
// hugo.custom_css is unset. The Relearn theme auto-loads that
// path after its own stylesheet, so this rule wins on every
// site that doesn't override hugo.custom_css explicitly.
//
// To replace this default, set hugo.custom_css in the user
// config. To opt out entirely, set it to a no-op value (e.g.,
// "/* no override */"); the pipeline writes the file with
// whatever the user supplies.
const defaultCustomCSS = `/* DocBuilder default override for the Relearn theme.
 *
 * The Relearn theme renders the sidebar's section titles
 * (.nav-title) at 2rem, bold, uppercase, with a 1rem left padding.
 * For documentation sites with many small category blocks, that
 * looks loud and ragged. These four values bring it back to a
 * section-divider weight: title case, 1.25rem, bold, 0.5rem left
 * padding.
 */
#R-sidebar .nav-title {
  font-size: 1.25rem;
  font-weight: bold;
  padding-inline-start: 0.5rem;
  text-transform: none;
}
`
