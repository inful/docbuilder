package version

// Version contains the application version information.
// This should be set via build-time ldflags in production:
// go build -ldflags "-X git.home.luguber.info/inful/docbuilder/internal/version.Version=v2.1.0".
var Version = "unknown"

// BuildInfo contains additional build metadata.
var (
	BuildTime = "unknown"
	GitCommit = "unknown"
)
