package git

import (
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// GitError simplifies creating a git-scoped ClassifiedError.
func GitError(message string) *errors.ErrorBuilder {
	return errors.NewError(errors.CategoryGit, message)
}

// ClassifyGitError translates go-git or command-line git errors into ClassifiedErrors.
func ClassifyGitError(err error, op string, url string) error {
	if err == nil {
		return nil
	}

	// Already classified
	if _, ok := errors.AsClassified(err); ok {
		return err
	}

	msg := err.Error()
	l := strings.ToLower(msg)

	builder := GitError("git operation failed").
		WithCause(err).
		WithContext("op", op).
		WithContext("url", url)

	switch {
	case strings.Contains(l, "authentication failed") || strings.Contains(l, "not authorized") || strings.Contains(l, "could not read username") || strings.Contains(l, "invalid credentials"):
		builder.WithCategory(errors.CategoryAuth)
	case strings.Contains(l, "repository not found") || strings.Contains(l, "not found") || strings.Contains(l, "does not exist"):
		builder.WithCategory(errors.CategoryNotFound)
	case strings.Contains(l, "remote hung up") || strings.Contains(l, "connection reset") || strings.Contains(l, "timeout") || strings.Contains(l, "i/o timeout") || strings.Contains(l, "no route to host"):
		builder.WithCategory(errors.CategoryNetwork).Retryable()
	case strings.Contains(l, "rate limit") || strings.Contains(l, "too many requests"):
		builder.WithCategory(errors.CategoryNetwork).RateLimit()
	case strings.Contains(l, "diverged") || strings.Contains(l, "non-fast-forward"):
		builder.WithContext("diverged", true)
	case strings.Contains(l, "unsupported protocol") || strings.Contains(l, "protocol not supported"):
		builder.WithCategory(errors.CategoryConfig)
	}

	return builder.Build()
}
