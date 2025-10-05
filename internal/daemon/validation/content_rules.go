package validation

import (
	"os"
	"path/filepath"
	"strings"
)

// ContentIntegrityRule validates content directory integrity when there were files in the previous build.
type ContentIntegrityRule struct{}

func (r ContentIntegrityRule) Name() string { return "content_integrity" }

func (r ContentIntegrityRule) Validate(ctx ValidationContext) ValidationResult {
	// Only validate if there were files in the previous build
	if ctx.PrevReport == nil || ctx.PrevReport.Files == 0 {
		return Success() // Skip validation for empty previous builds
	}

	contentDir := filepath.Join(ctx.OutDir, "content")

	// Check if content directory exists and is a directory
	contentStat, err := os.Stat(contentDir)
	if err != nil {
		return Failure("content directory missing")
	}
	if !contentStat.IsDir() {
		return Failure("content path is not a directory")
	}

	// Probe for at least one markdown file
	foundMD := false
	filepath.Walk(contentDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || foundMD || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			foundMD = true
		}
		return nil
	})

	if !foundMD {
		return Failure("no markdown files found in content directory")
	}

	return Success()
}

// GlobalDocHashRule validates global document files hash consistency.
type GlobalDocHashRule struct{}

func (r GlobalDocHashRule) Name() string { return "global_doc_hash" }

func (r GlobalDocHashRule) Validate(ctx ValidationContext) ValidationResult {
	// Only validate if there were files in the previous build
	if ctx.PrevReport == nil || ctx.PrevReport.Files == 0 {
		return Success()
	}

	lastGlobal := ctx.State.GetLastGlobalDocFilesHash()
	reportHash := ctx.PrevReport.DocFilesHash

	if lastGlobal != "" && reportHash != "" && lastGlobal != reportHash {
		return Failure("stored global doc_files_hash mismatch with report")
	}

	return Success()
}

// PerRepoDocHashRule validates per-repository document hash consistency.
type PerRepoDocHashRule struct{}

func (r PerRepoDocHashRule) Name() string { return "per_repo_doc_hash" }

func (r PerRepoDocHashRule) Validate(ctx ValidationContext) ValidationResult {
	// Only validate if there were files in the previous build
	if ctx.PrevReport == nil || ctx.PrevReport.Files == 0 {
		return Success()
	}

	// Handle single repository case
	if len(ctx.Repos) == 1 {
		repo := ctx.Repos[0]
		repoHash := ctx.State.GetRepoDocFilesHash(repo.URL)
		reportHash := ctx.PrevReport.DocFilesHash

		if repoHash == "" {
			return Failure("single repository doc_files_hash missing")
		}
		if reportHash != "" && repoHash != reportHash {
			return Failure("single repository doc_files_hash mismatch with report")
		}
		return Success()
	}

	// Handle multiple repositories case
	for _, repo := range ctx.Repos {
		if ctx.State.GetRepoDocFilesHash(repo.URL) == "" {
			return Failure("missing per-repo doc_files_hash for " + repo.URL)
		}
	}

	return Success()
}

// CommitMetadataRule validates that all repositories have last commit metadata.
type CommitMetadataRule struct{}

func (r CommitMetadataRule) Name() string { return "commit_metadata" }

func (r CommitMetadataRule) Validate(ctx ValidationContext) ValidationResult {
	for _, repo := range ctx.Repos {
		if ctx.State.GetRepoLastCommit(repo.URL) == "" {
			return Failure("missing last commit metadata for " + repo.URL)
		}
	}
	return Success()
}
