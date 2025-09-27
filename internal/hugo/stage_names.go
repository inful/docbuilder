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
