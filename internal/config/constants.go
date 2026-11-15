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
)
