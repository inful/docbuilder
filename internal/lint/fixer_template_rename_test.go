package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fixer renames a template file and binds the uid of its body: the
// binding ran on the old name, the file was not there any more, and the
// fix was lost.
func TestTheRenamedTemplateFileGetsItsUIDBinding(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "My Template.md")
	content := wrappingTemplate("---\ntitle: New\n---\n\nSee [uid](/_uid/{{ .UID }}/).\n")
	require.NoError(t, os.WriteFile(name, []byte(content), 0o600))

	fixer := NewFixer(NewLinter(&Config{}), false, false)
	result, err := fixer.Fix(dir)
	require.NoError(t, err)
	require.Empty(t, result.Errors, "the fixer reported errors")

	renamed := filepath.Join(dir, "my-template.md")
	require.FileExists(t, renamed, "the template was not renamed")

	// #nosec G304 -- the path is the file of the test.
	data, err := os.ReadFile(renamed)
	require.NoError(t, err)
	require.Contains(t, string(data), "{{ .UID }}",
		"the uid of the template was not bound to the body")
}
