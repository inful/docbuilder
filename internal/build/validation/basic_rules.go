package validation

import (
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// BasicPrerequisitesRule validates basic prerequisites for skip evaluation.
type BasicPrerequisitesRule struct{}

func (r BasicPrerequisitesRule) Name() string { return "basic_prerequisites" }

func (r BasicPrerequisitesRule) Validate(ctx Context) Result {
	if ctx.State == nil {
		return Failure("state manager is nil")
	}
	if ctx.Generator == nil {
		return Failure("generator is nil")
	}
	if len(ctx.Repos) == 0 {
		return Failure("no repositories to process")
	}
	return Success()
}

// ConfigHashRule validates that the configuration hasn't changed.
type ConfigHashRule struct{}

func (r ConfigHashRule) Name() string { return "config_hash" }

func (r ConfigHashRule) Validate(ctx Context) Result {
	currentHash := ctx.Generator.ComputeConfigHashForPersistence()
	if currentHash == "" {
		return Failure("current config hash is empty")
	}

	storedHash := ctx.State.GetLastConfigHash()
	if currentHash != storedHash {
		return Failure("config hash mismatch")
	}

	return Success()
}

// PublicDirectoryRule validates that the public output directory exists and is valid.
type PublicDirectoryRule struct{}

func (r PublicDirectoryRule) Name() string { return "public_directory" }

func (r PublicDirectoryRule) Validate(ctx Context) Result {
	publicDir := filepath.Join(ctx.OutDir, "public")

	// Check if directory exists and is a directory
	fi, err := os.Stat(publicDir)
	if err != nil {
		return Failure("public directory missing")
	}
	if !fi.IsDir() {
		return Failure("public path is not a directory")
	}

	// Check if directory has content
	entries, err := os.ReadDir(publicDir)
	if err != nil {
		return Failure("failed to read public directory")
	}
	if len(entries) == 0 {
		return Failure("public directory is empty")
	}

	return Success()
}

// VersionMismatchRule validates that DocBuilder and Hugo versions haven't changed.
// If either version differs from the previous build, a rebuild is forced to ensure
// compatibility and that new features/fixes take effect.
type VersionMismatchRule struct{}

func (r VersionMismatchRule) Name() string { return "version_mismatch" }

func (r VersionMismatchRule) Validate(ctx Context) Result {
	if ctx.PrevReport == nil {
		return Failure("no previous report available")
	}

	// Check DocBuilder version
	currentDocBuilderVersion := version.Version
	if currentDocBuilderVersion != ctx.PrevReport.DocBuilderVersion {
		return Failure("docbuilder version changed")
	}

	// Check Hugo version (only if Hugo was used in previous build)
	// Empty previous Hugo version means Hugo wasn't executed
	if ctx.PrevReport.HugoVersion != "" {
		currentHugoVersion := hugo.DetectHugoVersion(ctx.Context)
		if currentHugoVersion != ctx.PrevReport.HugoVersion {
			return Failure("hugo version changed")
		}
	}

	return Success()
}
