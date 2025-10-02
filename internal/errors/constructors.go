package errors

// Convenience functions for common error patterns

// Config errors
func ConfigNotFound(path string) *DocBuilderError {
	return New(CategoryConfig, SeverityFatal, "configuration file not found").
		WithContext("path", path)
}

func ConfigInvalid(msg string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryConfig, SeverityFatal, msg)
}

func ValidationFailed(field, reason string) *DocBuilderError {
	return New(CategoryValidation, SeverityFatal, "validation failed").
		WithContext("field", field).
		WithContext("reason", reason)
}

// Auth errors
func AuthFailed(operation, method string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryAuth, SeverityFatal, "authentication failed").
		WithContext("operation", operation).
		WithContext("method", method)
}

func AuthConfigMissing(forge string) *DocBuilderError {
	return New(CategoryAuth, SeverityFatal, "authentication configuration missing").
		WithContext("forge", forge)
}

// Network errors
func NetworkTimeout(url string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryNetwork, SeverityWarning, "network timeout").
		WithContext("url", url)
}

func NetworkConnectionFailed(url string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryNetwork, SeverityWarning, "connection failed").
		WithContext("url", url)
}

// Git errors
func GitCloneFailed(repo, reason string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryGit, SeverityWarning, "git clone failed").
		WithContext("repository", repo).
		WithContext("reason", reason)
}

func GitAuthFailed(repo string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryGit, SeverityFatal, "git authentication failed").
		WithContext("repository", repo)
}

func GitRepoNotFound(repo string) *DocBuilderError {
	return New(CategoryGit, SeverityFatal, "repository not found").
		WithContext("repository", repo)
}

// Forge errors
func ForgeUnavailable(forge string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryForge, SeverityWarning, "forge API unavailable").
		WithContext("forge", forge)
}

func ForgeRateLimit(forge string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryForge, SeverityWarning, "forge API rate limit exceeded").
		WithContext("forge", forge)
}

// Build errors
func BuildFailed(stage, reason string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryBuild, SeverityFatal, "build failed").
		WithContext("stage", stage).
		WithContext("reason", reason)
}

func BuildWarning(stage, reason string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryBuild, SeverityWarning, "build warning").
		WithContext("stage", stage).
		WithContext("reason", reason)
}

// Hugo errors
func HugoFailed(command string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryHugo, SeverityFatal, "Hugo execution failed").
		WithContext("command", command)
}

func HugoConfigInvalid(reason string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryHugo, SeverityFatal, "Hugo configuration invalid").
		WithContext("reason", reason)
}

// Filesystem errors
func FileSystemError(operation, path string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryFileSystem, SeverityFatal, "filesystem operation failed").
		WithContext("operation", operation).
		WithContext("path", path)
}

func PermissionDenied(path string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryFileSystem, SeverityFatal, "permission denied").
		WithContext("path", path)
}

// Runtime errors
func RuntimeError(component, reason string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryRuntime, SeverityFatal, "runtime error").
		WithContext("component", component).
		WithContext("reason", reason)
}

// Daemon errors
func DaemonStartFailed(reason string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryDaemon, SeverityFatal, "daemon failed to start").
		WithContext("reason", reason)
}

func DaemonOperationFailed(operation string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryDaemon, SeverityFatal, "daemon operation failed").
		WithContext("operation", operation)
}

// Internal errors
func InternalError(component, reason string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryInternal, SeverityFatal, "internal error").
		WithContext("component", component).
		WithContext("reason", reason)
}
