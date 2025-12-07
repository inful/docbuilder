package errors

// Convenience functions for common error patterns

// Config errors

func ConfigNotFound(path string) *DocBuilderError {
	return New(CategoryConfig, SeverityFatal, "configuration file not found").
		WithContext("path", path)
}

func ConfigRequired(field string) *DocBuilderError {
	return New(CategoryConfig, SeverityFatal, "required configuration missing").
		WithContext("field", field)
}

func ValidationFailed(field, reason string) *DocBuilderError {
	return New(CategoryValidation, SeverityFatal, "validation failed").
		WithContext("field", field).
		WithContext("reason", reason)
}

// Build pipeline errors

func BuildFailed(stage string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryBuild, SeverityFatal, "build failed").
		WithContext("stage", stage)
}

func WorkspaceError(operation string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryFileSystem, SeverityFatal, "workspace operation failed").
		WithContext("operation", operation)
}

func DiscoveryError(cause error) *DocBuilderError {
	return Wrap(cause, CategoryBuild, SeverityFatal, "documentation discovery failed")
}

func HugoGenerationError(cause error) *DocBuilderError {
	return Wrap(cause, CategoryHugo, SeverityFatal, "Hugo site generation failed")
}

// Git errors

func GitCloneError(repo string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryGit, SeverityFatal, "repository clone failed").
		WithContext("repository", repo)
}

func GitAuthError(repo string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryAuth, SeverityFatal, "git authentication failed").
		WithContext("repository", repo)
}

func GitNetworkError(repo string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryGit, SeverityWarning, "git network error").
		WithContext("repository", repo)
}

// Network errors

func NetworkTimeout(url string, cause error) *DocBuilderError {
	return WrapRetryable(cause, CategoryNetwork, SeverityWarning, "network timeout").
		WithContext("url", url)
}

// Internal errors

func InternalError(message string, cause error) *DocBuilderError {
	return Wrap(cause, CategoryInternal, SeverityFatal, message)
}
