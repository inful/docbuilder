package validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ContentIntegrityRule validates content directory integrity when there were files in the previous build.
type ContentIntegrityRule struct{}

func (r ContentIntegrityRule) Name() string { return "content_integrity" }

func (r ContentIntegrityRule) Validate(ctx context.Context, vctx Context) Result {
	// Only validate if there were files in the previous build
	if vctx.PrevReport == nil || vctx.PrevReport.Files == 0 {
		return Success() // Skip validation for empty previous builds
	}

	contentDir := filepath.Join(vctx.OutDir, "content")

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
	if werr := filepath.Walk(contentDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if foundMD || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			foundMD = true
		}
		return nil
	}); werr != nil {
		return Failure("error scanning content directory: " + werr.Error())
	}

	if !foundMD {
		return Failure("no markdown files found in content directory")
	}

	return Success()
}

// GlobalDocHashRule validates global document files hash consistency.
type GlobalDocHashRule struct{}

func (r GlobalDocHashRule) Name() string { return "global_doc_hash" }

func (r GlobalDocHashRule) Validate(ctx context.Context, vctx Context) Result {
	// Only validate if there were files in the previous build
	if vctx.PrevReport == nil || vctx.PrevReport.Files == 0 {
		return Success()
	}

	lastGlobal := vctx.State.GetLastGlobalDocFilesHash()
	reportHash := vctx.PrevReport.DocFilesHash

	if lastGlobal != "" && reportHash != "" && lastGlobal != reportHash {
		return Failure("stored global doc_files_hash mismatch with report")
	}

	return Success()
}

// PerRepoDocHashRule validates per-repository document hash consistency.
type PerRepoDocHashRule struct{}

func (r PerRepoDocHashRule) Name() string { return "per_repo_doc_hash" }

func (r PerRepoDocHashRule) Validate(ctx context.Context, vctx Context) Result {
	// Only validate if there were files in the previous build
	if vctx.PrevReport == nil || vctx.PrevReport.Files == 0 {
		return Success()
	}

	// Handle single repository case
	if len(vctx.Repos) == 1 {
		repo := vctx.Repos[0]
		repoHash := vctx.State.GetRepoDocFilesHash(repo.URL)
		reportHash := vctx.PrevReport.DocFilesHash

		if repoHash == "" {
			return Failure("single repository doc_files_hash missing")
		}
		if reportHash != "" && repoHash != reportHash {
			return Failure("single repository doc_files_hash mismatch with report")
		}
		return Success()
	}

	// Handle multiple repositories case
	for i := range vctx.Repos {
		repo := &vctx.Repos[i]
		if vctx.State.GetRepoDocFilesHash(repo.URL) == "" {
			return Failure("missing per-repo doc_files_hash for " + repo.URL)
		}
	}

	return Success()
}

// CommitMetadataRule validates that all repositories have last commit metadata.
type CommitMetadataRule struct{}

func (r CommitMetadataRule) Name() string { return "commit_metadata" }

func (r CommitMetadataRule) Validate(ctx context.Context, vctx Context) Result {
	for i := range vctx.Repos {
		repo := &vctx.Repos[i]
		if vctx.State.GetRepoLastCommit(repo.URL) == "" {
			return Failure("missing last commit metadata for " + repo.URL)
		}
	}
	return Success()
}
