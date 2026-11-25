package lint

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The root of the file system is not a root: the paths are in
// different trees, and the scan of the broken links from there walks
// the whole disk.
func TestTheCommonRootIsNotTheRootOfTheFileSystem(t *testing.T) {
	root := commonRoot([]string{t.TempDir(), "/etc"})

	require.NotEqual(t, string(filepath.Separator), root,
		"the scan of the broken links would walk the whole disk")
}
