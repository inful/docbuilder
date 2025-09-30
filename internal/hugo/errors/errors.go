package errors

// Package errors provides sentinel errors for Hugo site generation stages.
// These enable consistent classification (expanded in Phase 4) while keeping
// user‑facing messages descriptive via wrapping.

import "errors"

var (
	// ErrHugoBinaryNotFound indicates the hugo executable was not detected on PATH.
	ErrHugoBinaryNotFound = errors.New("hugo binary not found")
	// ErrHugoExecutionFailed indicates the hugo command returned a non‑zero exit status.
	ErrHugoExecutionFailed = errors.New("hugo execution failed")
	// ErrConfigMarshalFailed indicates marshaling the generated Hugo configuration failed.
	ErrConfigMarshalFailed = errors.New("hugo config marshal failed")
	// ErrConfigWriteFailed indicates writing the hugo.yaml file failed.
	ErrConfigWriteFailed = errors.New("hugo config write failed")
	
	// Generation stage errors
	// ErrContentTransformFailed indicates a content transformer failed during pipeline execution.
	ErrContentTransformFailed = errors.New("content transform failed")
	// ErrContentWriteFailed indicates writing processed content to the Hugo content directory failed.
	ErrContentWriteFailed = errors.New("content write failed")
	// ErrIndexGenerationFailed indicates generating index files (main, repository, section) failed.
	ErrIndexGenerationFailed = errors.New("index generation failed")
	// ErrLayoutCopyFailed indicates copying theme layouts to the Hugo site failed.
	ErrLayoutCopyFailed = errors.New("layout copy failed")
	// ErrStagingFailed indicates staging directory operations failed.
	ErrStagingFailed = errors.New("staging operation failed")
	// ErrReportPersistFailed indicates writing the build report failed.
	ErrReportPersistFailed = errors.New("build report persist failed")
)
