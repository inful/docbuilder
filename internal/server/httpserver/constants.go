package httpserver

const (
	logLevelWarn  = "warn"
	logLevelError = "error"

	cacheControlWeek       = "public, max-age=604800"
	cacheControlDay        = "public, max-age=86400"
	cacheControlFiveMinute = "public, max-age=300"
	cacheControlHour       = "public, max-age=3600"
	cacheControlNoCache    = "no-cache, must-revalidate"
)
