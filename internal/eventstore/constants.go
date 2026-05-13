package eventstore

const (
	eventTypeBuildStarted        = "BuildStarted"
	eventTypeRepositoryCloned    = "RepositoryCloned"
	eventTypeDocumentsDiscovered = "DocumentsDiscovered"
	eventTypeTransformApplied    = "TransformApplied"
	eventTypeHugoConfigGenerated = "HugoConfigGenerated"
	eventTypeSiteGenerated       = "SiteGenerated"
	eventTypeBuildCompleted      = "BuildCompleted"
	eventTypeBuildFailed         = "BuildFailed"

	payloadKeyDurationMS = "duration_ms"
	payloadKeyFileCount  = "file_count"
)
