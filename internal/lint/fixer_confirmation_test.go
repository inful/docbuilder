package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// answerIs puts the answer of the operator in the place the fixer reads
// it from.
func answerIs(t *testing.T, answer string) {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	_, err = writer.WriteString(answer + "\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	stdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() { os.Stdin = stdin })
}

// The fixer asks before it changes the files: the files of the operator
// are changed, and the operator has to approve what is changed.
func TestTheFixerAsksBeforeItChangesTheFiles(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "Bad Name.md")
	require.NoError(t, os.WriteFile(page, []byte("# Bad\n"), 0o600))

	answerIs(t, "n")

	fixer := NewFixer(NewLinter(&Config{}), false, false)
	_, err := fixer.FixPathsWithConfirmation([]string{dir})
	require.Error(t, err, "the operator did not approve the changes")

	// The file is not renamed.
	_, err = os.Stat(page)
	require.NoError(t, err, "the file was renamed without the approval of the operator")
}

// The answer of the operator is asked for, and the changes are applied
// when the operator approves them.
func TestTheChangesAreAppliedWhenTheOperatorApprovesThem(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "Bad Name.md")
	require.NoError(t, os.WriteFile(page, []byte("# Bad\n"), 0o600))

	answerIs(t, "y")

	fixer := NewFixer(NewLinter(&Config{}), false, false)
	_, err := fixer.FixPathsWithConfirmation([]string{dir})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "bad-name.md"))
	require.NoError(t, err, "the file was not renamed")
}

// The confirmation is skipped when the answer is already known: the
// run of a script has nobody to ask.
func TestTheConfirmationIsSkippedWhenTheAnswerIsKnown(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "Bad Name.md")
	require.NoError(t, os.WriteFile(page, []byte("# Bad\n"), 0o600))

	fixer := NewFixer(NewLinter(&Config{}), false, false).WithAutoConfirm(true)
	_, err := fixer.FixPathsWithConfirmation([]string{dir})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "bad-name.md"))
	require.NoError(t, err, "the file was not renamed")
}
