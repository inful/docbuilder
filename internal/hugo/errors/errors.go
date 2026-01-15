package errors

// Package errors provides sentinel errors for Hugo site generation stages.
// These are classified errors that enable consistent error handling throughout
// the Hugo generation pipeline.

import foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"

var (
	// ErrHugoBinaryNotFound indicates the hugo executable was not detected on PATH.
	ErrHugoBinaryNotFound = foundationerrors.NewError(foundationerrors.CategoryHugo, "hugo binary not found").Build()
	// ErrHugoExecutionFailed indicates the hugo command returned a non‑zero exit status.
	ErrHugoExecutionFailed = foundationerrors.NewError(foundationerrors.CategoryHugo, "hugo execution failed").Build()
	// ErrConfigMarshalFailed indicates marshaling the generated Hugo configuration failed.
	ErrConfigMarshalFailed = foundationerrors.NewError(foundationerrors.CategoryBuild, "hugo config marshal failed").Build()
	// ErrConfigWriteFailed indicates writing the hugo.yaml file failed.
	ErrConfigWriteFailed = foundationerrors.NewError(foundationerrors.CategoryFileSystem, "hugo config write failed").Build()

	// ErrContentTransformFailed indicates a content transformer failed during pipeline execution.
	ErrContentTransformFailed = foundationerrors.NewError(foundationerrors.CategoryBuild, "content transform failed").Build()
	// ErrContentWriteFailed indicates writing processed content to the Hugo content directory failed.
	ErrContentWriteFailed = foundationerrors.NewError(foundationerrors.CategoryFileSystem, "content write failed").Build()
	// ErrIndexGenerationFailed indicates generating index files (main, repository, section) failed.
	ErrIndexGenerationFailed = foundationerrors.NewError(foundationerrors.CategoryBuild, "index generation failed").Build()
	// ErrLayoutCopyFailed indicates copying theme layouts to the Hugo site failed.
	ErrLayoutCopyFailed = foundationerrors.NewError(foundationerrors.CategoryFileSystem, "layout copy failed").Build()
	// ErrStagingFailed indicates staging directory operations failed.
	ErrStagingFailed = foundationerrors.NewError(foundationerrors.CategoryFileSystem, "staging operation failed").Build()
	// ErrReportPersistFailed indicates writing the build report failed.
	ErrReportPersistFailed = foundationerrors.NewError(foundationerrors.CategoryFileSystem, "build report persist failed").Build()
)
