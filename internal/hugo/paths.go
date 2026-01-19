package hugo

// Centralized path helpers to avoid scattering staging vs final root logic.
// All writes that should occur during a build (pre-promotion) must use buildRoot().
// Any read after promotion or side-effect targeting the final site should use finalRoot().

// BuildRoot returns the directory that active build stages should write into (staging if present, else final output).
func (g *Generator) BuildRoot() string {
	if g.stageDir != "" {
		return g.stageDir
	}
	return g.outputDir
}

// finalRoot returns the final output directory path (even if staging is active).
func (g *Generator) finalRoot() string { return g.outputDir }
