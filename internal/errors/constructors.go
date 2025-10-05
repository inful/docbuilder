package errors

// Convenience functions for common error patterns

// Config errors
func ConfigNotFound(path string) *DocBuilderError {
	return New(CategoryConfig, SeverityFatal, "configuration file not found").
		WithContext("path", path)
}

// Deprecated: ConfigInvalid is unused; prefer returning rich context via ValidationFailed.
// Keeping for backward compatibility if external tools referenced it previously (not exported outside module).
// func ConfigInvalid(msg string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryConfig, SeverityFatal, msg)
// }

func ValidationFailed(field, reason string) *DocBuilderError {
	return New(CategoryValidation, SeverityFatal, "validation failed").
		WithContext("field", field).
		WithContext("reason", reason)
}

// Auth errors
// Deprecated: AuthFailed is unused; prefer adapters translating from foundation errors.
// func AuthFailed(operation, method string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryAuth, SeverityFatal, "authentication failed").
// 		WithContext("operation", operation).
// 		WithContext("method", method)
// }

// Deprecated: AuthConfigMissing is unused.
// func AuthConfigMissing(forge string) *DocBuilderError {
// 	return New(CategoryAuth, SeverityFatal, "authentication configuration missing").
// 		WithContext("forge", forge)
// }

// Network errors
func NetworkTimeout(url string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryNetwork, SeverityWarning, "network timeout").
		WithContext("url", url)
}

// Deprecated: NetworkConnectionFailed is unused.
// func NetworkConnectionFailed(url string, cause error) *DocBuilderError {
// 	return WrapRetryable(cause, CategoryNetwork, SeverityWarning, "connection failed").
// 		WithContext("url", url)
// }

// Git errors
// Deprecated: GitCloneFailed is unused.
// func GitCloneFailed(repo, reason string, cause error) *DocBuilderError {
// 	return WrapRetryable(cause, CategoryGit, SeverityWarning, "git clone failed").
// 		WithContext("repository", repo).
// 		WithContext("reason", reason)
// }

// Deprecated: GitAuthFailed is unused.
// func GitAuthFailed(repo string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryGit, SeverityFatal, "git authentication failed").
// 		WithContext("repository", repo)
// }

// Deprecated: GitRepoNotFound is unused.
// func GitRepoNotFound(repo string) *DocBuilderError {
// 	return New(CategoryGit, SeverityFatal, "repository not found").
// 		WithContext("repository", repo)
// }

// Forge errors
// Deprecated: ForgeUnavailable is unused.
// func ForgeUnavailable(forge string, cause error) *DocBuilderError {
// 	return WrapRetryable(cause, CategoryForge, SeverityWarning, "forge API unavailable").
// 		WithContext("forge", forge)
// }

// Deprecated: ForgeRateLimit is unused.
// func ForgeRateLimit(forge string, cause error) *DocBuilderError {
// 	return WrapRetryable(cause, CategoryForge, SeverityWarning, "forge API rate limit exceeded").
// 		WithContext("forge", forge)
// }

// Build errors
// Deprecated: BuildFailed is unused.
// func BuildFailed(stage, reason string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryBuild, SeverityFatal, "build failed").
// 		WithContext("stage", stage).
// 		WithContext("reason", reason)
// }

// Deprecated: BuildWarning is unused.
// func BuildWarning(stage, reason string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryBuild, SeverityWarning, "build warning").
// 		WithContext("stage", stage).
// 		WithContext("reason", reason)
// }

// Hugo errors
// Deprecated: HugoFailed is unused.
// func HugoFailed(command string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryHugo, SeverityFatal, "Hugo execution failed").
// 		WithContext("command", command)
// }

// Deprecated: HugoConfigInvalid is unused.
// func HugoConfigInvalid(reason string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryHugo, SeverityFatal, "Hugo configuration invalid").
// 		WithContext("reason", reason)
// }

// Filesystem errors
// Deprecated: FileSystemError is unused.
// func FileSystemError(operation, path string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryFileSystem, SeverityFatal, "filesystem operation failed").
// 		WithContext("operation", operation).
// 		WithContext("path", path)
// }

// Deprecated: PermissionDenied is unused.
// func PermissionDenied(path string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryFileSystem, SeverityFatal, "permission denied").
// 		WithContext("path", path)
// }

// Runtime errors
// Deprecated: RuntimeError is unused.
// func RuntimeError(component, reason string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryRuntime, SeverityFatal, "runtime error").
// 		WithContext("component", component).
// 		WithContext("reason", reason)
// }

// Daemon errors
// Deprecated: DaemonStartFailed is unused.
// func DaemonStartFailed(reason string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryDaemon, SeverityFatal, "daemon failed to start").
// 		WithContext("reason", reason)
// }

// Deprecated: DaemonOperationFailed is unused.
// func DaemonOperationFailed(operation string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryDaemon, SeverityFatal, "daemon operation failed").
// 		WithContext("operation", operation)
// }

// Internal errors
// Deprecated: InternalError is unused.
// func InternalError(component, reason string, cause error) *DocBuilderError {
// 	return Wrap(cause, CategoryInternal, SeverityFatal, "internal error").
// 		WithContext("component", component).
// 		WithContext("reason", reason)
// }
