package errors

// Package errors provides sentinel errors for documentation discovery operations.
// These enable consistent classification and improved error handling for docs stage failures.

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

var (
	// ErrDocsPathNotFound indicates a configured documentation path does not exist in the repository.
	ErrDocsPathNotFound = errors.DocsError("documentation path not found").Build()

	// ErrDocsDirWalkFailed indicates filesystem traversal of a docs directory failed.
	ErrDocsDirWalkFailed = errors.DocsError("documentation directory walk failed").Build()

	// ErrFileReadFailed indicates reading content from a discovered documentation file failed.
	ErrFileReadFailed = errors.DocsError("documentation file read failed").Build()

	// ErrDocIgnoreCheckFailed indicates checking for .docignore file failed.
	ErrDocIgnoreCheckFailed = errors.DocsError("docignore check failed").Build()

	// ErrNoDocsFound indicates no documentation files were discovered in any repository.
	ErrNoDocsFound = errors.DocsError("no documentation files found").Build()

	// ErrInvalidRelativePath indicates calculating relative path from docs base failed.
	ErrInvalidRelativePath = errors.DocsError("invalid relative path calculation").Build()

	// ErrPathCollision indicates multiple source files map to the same Hugo path due to case normalization.
	ErrPathCollision = errors.DocsError("path collision detected").Build()
)
