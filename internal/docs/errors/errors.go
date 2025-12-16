package errors

// Package errors provides sentinel errors for documentation discovery operations.
// These enable consistent classification and improved error handling for docs stage failures.

import "errors"

var (
	// ErrDocsPathNotFound indicates a configured documentation path does not exist in the repository.
	ErrDocsPathNotFound = errors.New("documentation path not found")

	// ErrDocsDirWalkFailed indicates filesystem traversal of a docs directory failed.
	ErrDocsDirWalkFailed = errors.New("documentation directory walk failed")

	// ErrFileReadFailed indicates reading content from a discovered documentation file failed.
	ErrFileReadFailed = errors.New("documentation file read failed")

	// ErrDocIgnoreCheckFailed indicates checking for .docignore file failed.
	ErrDocIgnoreCheckFailed = errors.New("docignore check failed")

	// ErrNoDocsFound indicates no documentation files were discovered in any repository.
	ErrNoDocsFound = errors.New("no documentation files found")

	// ErrInvalidRelativePath indicates calculating relative path from docs base failed.
	ErrInvalidRelativePath = errors.New("invalid relative path calculation")

	// ErrPathCollision indicates multiple source files map to the same Hugo path due to case normalization.
	ErrPathCollision = errors.New("path collision detected")
)
