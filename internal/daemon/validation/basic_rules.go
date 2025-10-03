package validation

import (
	"os"
	"path/filepath"
)

// BasicPrerequisitesRule validates basic prerequisites for skip evaluation.
type BasicPrerequisitesRule struct{}

func (r BasicPrerequisitesRule) Name() string { return "basic_prerequisites" }

func (r BasicPrerequisitesRule) Validate(ctx ValidationContext) ValidationResult {
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

func (r ConfigHashRule) Validate(ctx ValidationContext) ValidationResult {
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

func (r PublicDirectoryRule) Validate(ctx ValidationContext) ValidationResult {
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