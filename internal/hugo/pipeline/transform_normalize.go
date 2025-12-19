package pipeline

import (
	"path/filepath"
	"strings"
)

// normalizeIndexFiles renames README files to _index for Hugo compatibility.
// This must run early before other transforms depend on the file name.
func normalizeIndexFiles(doc *Document) ([]*Document, error) {
	// Check if this is a README file at any level
	if strings.EqualFold(doc.Name, "README") {
		// Rename to _index for Hugo
		// Update both Name and Path
		doc.Name = "_index"

		// Update Path to reflect the new name
		dir := filepath.Dir(doc.Path)
		if dir == "." {
			doc.Path = "_index" + doc.Extension
		} else {
			doc.Path = filepath.Join(dir, "_index"+doc.Extension)
		}
	}

	return nil, nil
}
