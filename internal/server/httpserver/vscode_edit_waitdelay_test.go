package httpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The VS Code CLI can leave a process behind that holds its output open.
//
// The command was then never finished: the request hung forever, the
// browser waited forever, and the "Edit this page" link did nothing.
// The command itself was done - it exited with a zero status - and the
// file was opened all the same.
func TestTheCommandIsNotHeldByAProcessThatHoldsTheOutput(t *testing.T) {
	dir := t.TempDir()

	// A script that leaves a background process behind: the process
	// holds the output of the command open.
	script := filepath.Join(dir, "code")
	content := "#!/bin/sh\n(sleep 30 &)\nexit 0\n"
	require.NoError(t, os.WriteFile(script, []byte(content), 0o700)) // #nosec G306 -- the test runs it

	srv := &Server{}
	srv.vscodeFindCLI = func(ctx context.Context) string { return script }
	srv.vscodeFindIPCSocket = func() string { return "/tmp/vscode-ipc-test.sock" }
	srv.vscodeOpenBackoffs = []time.Duration{}

	start := time.Now()

	_, _, err := runVSCodeCLI(t.Context(), script, []string{"--goto", "/tmp/x.md"}, os.Environ())

	elapsed := time.Since(start)

	require.NoError(t, err, "the command itself was done")
	require.Less(t, elapsed, 20*time.Second, "the command was held by the process behind it")
}
