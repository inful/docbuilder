package pipeline

import (
	"path/filepath"
	"strings"
)

// normalizeIndexFiles renames README files to _index for Hugo compatibility.
// This must run early before other transforms depend on the file name.
func normalizeIndexFiles(doc *Document) ([]*Document, error) {
	// The directory has an index of its own: the README is a page of
	// its own, and renaming it would write it to the page of the index.
	if doc.KeepName {
		return nil, nil
	}

	// Check if this is a README file at any level
	if strings.EqualFold(doc.Name, "README") {
		// Rename to _index for Hugo
		// Update both Name and Path
		doc.Name = indexFileSuffix

		// Update Path to reflect the new name
		dir := filepath.Dir(doc.Path)
		if dir == "." {
			doc.Path = indexFileSuffix + doc.Extension
		} else {
			doc.Path = filepath.Join(dir, indexFileSuffix+doc.Extension)
		}
	}

	return nil, nil
}
