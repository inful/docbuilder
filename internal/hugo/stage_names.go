package hugo

// StageName is a strongly-typed identifier for a build stage. All canonical
// stages are declared as constants here for compile-time safety.
type StageName string

// Canonical stage names.
const (
    StagePrepareOutput  StageName = "prepare_output"
    StageCloneRepos     StageName = "clone_repos"
    StageDiscoverDocs   StageName = "discover_docs"
    StageGenerateConfig StageName = "generate_config"
    StageLayouts        StageName = "layouts"
    StageCopyContent    StageName = "copy_content"
    StageIndexes        StageName = "indexes"
    StageRunHugo        StageName = "run_hugo"
    StagePostProcess    StageName = "post_process"
)

// StageDef pairs a stage name with its executing function (internal wiring helper).
type StageDef struct {
    Name StageName
    Fn   Stage
}

// Pipeline is a fluent builder for ordered stage definitions.
// It enables conditional inclusion and future plugin insertion without
// manually assembling slices inline in generator methods.
type Pipeline struct { defs []StageDef }

// NewPipeline creates an empty pipeline.
func NewPipeline() *Pipeline { return &Pipeline{defs: make([]StageDef, 0, 8)} }

// Add appends a stage unconditionally.
func (p *Pipeline) Add(name StageName, fn Stage) *Pipeline {
    p.defs = append(p.defs, StageDef{Name: name, Fn: fn})
    return p
}

// AddIf appends a stage only if cond is true.
func (p *Pipeline) AddIf(cond bool, name StageName, fn Stage) *Pipeline {
    if cond { p.Add(name, fn) }
    return p
}

// Build returns a defensive copy of the stage definitions slice.
func (p *Pipeline) Build() []StageDef {
    out := make([]StageDef, len(p.defs))
    copy(out, p.defs)
    return out
}
