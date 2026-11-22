package config

const (
	defaultRequiredPathDocs    = "docs"
	defaultIgnoreFileDocignore = ".docignore"
	defaultMetricsPath         = "/metrics"
	defaultSyncScheduleEvery4h = "0 */4 * * *"
	forgeOptionAutoDiscoverKey = "auto_discover"

	// defaultAdminBind keeps the admin server on the loopback interface.
	// The admin API has no password, so it must not be reachable from the
	// network by accident.
	defaultAdminBind = "127.0.0.1"

	// defaultEventRetention is how long build events are kept in the event
	// store. Without it the event log grows without bound.
	defaultEventRetention = "720h" // 30 days

	// defaultStateDir is where the daemon keeps its state. It is separate
	// from the repository cache so the two do not share a directory.
	defaultStateDir = "./daemon-data"
)

// DefaultStateDir is where the daemon keeps its state when the config does
// not say where.
const DefaultStateDir = defaultStateDir
