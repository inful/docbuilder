package hugo

import (
    "fmt"
    "log/slog"
    "os"
    "os/exec"
)

// shouldRunHugo determines if we should invoke the external hugo binary.
// Enabled when DOCBUILDER_RUN_HUGO=1 and hugo binary exists in PATH, unless DOCBUILDER_SKIP_HUGO=1.
func shouldRunHugo() bool {
    if os.Getenv("DOCBUILDER_SKIP_HUGO") == "1" { return false }
    if os.Getenv("DOCBUILDER_RUN_HUGO") != "1" { return false }
    _, err := exec.LookPath("hugo")
    return err == nil
}

// runHugoBuild executes `hugo` inside the output directory to produce the static site under public/.
func (g *Generator) runHugoBuild() error {
    cmd := exec.Command("hugo")
    cmd.Dir = g.finalRoot() // run only against finalized site
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    slog.Info("Running Hugo binary to render static site", "dir", g.finalRoot())
    if err := cmd.Run(); err != nil { return fmt.Errorf("hugo command failed: %w", err) }
    return nil
}
