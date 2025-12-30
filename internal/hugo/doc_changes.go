package hugo

import "git.home.luguber.info/inful/docbuilder/internal/docs"

// DetectDocumentChanges checks if documentation files have changed between builds.
func DetectDocumentChanges(prevFiles, newFiles []docs.DocFile) bool {
	prevCount := len(prevFiles)
	if prevCount == 0 {
		return false
	}

	// Quick count check
	if len(newFiles) != prevCount {
		return true
	}

	// Build sets for comparison
	prevSet := buildFilePathSet(prevFiles)
	nowSet := buildFilePathSet(newFiles)

	// Check for new files
	if hasNewFiles(nowSet, prevSet) {
		return true
	}

	// Check for removed files
	return hasRemovedFiles(prevSet, nowSet)
}

// buildFilePathSet creates a set of Hugo paths from doc files.
func buildFilePathSet(files []docs.DocFile) map[string]struct{} {
	set := make(map[string]struct{}, len(files))
	for _, f := range files {
		set[f.GetHugoPath()] = struct{}{}
	}
	return set
}

// hasNewFiles checks if there are any files in nowSet that aren't in prevSet.
func hasNewFiles(nowSet, prevSet map[string]struct{}) bool {
	for path := range nowSet {
		if _, exists := prevSet[path]; !exists {
			return true
		}
	}
	return false
}

// hasRemovedFiles checks if there are any files in prevSet that aren't in nowSet.
func hasRemovedFiles(prevSet, nowSet map[string]struct{}) bool {
	for path := range prevSet {
		if _, exists := nowSet[path]; !exists {
			return true
		}
	}
	return false
}
