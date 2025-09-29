package hugo

import (
	"fmt"
	"log/slog"
	"os/exec"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
)

// Renderer abstracts how the final static site rendering step is performed after
// Hugo project scaffolding. This allows swapping out the external hugo binary
// (BinaryRenderer) with alternative strategies (e.g., no-op for tests, remote
// render service, in-process library) without changing stage orchestration.
//
// Contract:
//
//	Execute(rootDir string) error  -> perform rendering inside provided directory.
//	Enabled(cfg *config.Config) bool -> determines if rendering should run (allows
//	  renderer-level gating beyond global build.render_mode semantics)
//
// Errors returned are surfaced as warnings (non-fatal) unless future policy changes.
type Renderer interface {
	Execute(rootDir string) error
}

// BinaryRenderer invokes the `hugo` binary present on PATH.
type BinaryRenderer struct{}

func (b *BinaryRenderer) Execute(rootDir string) error {
	if _, err := exec.LookPath("hugo"); err != nil {
		return fmt.Errorf("%w: %v", herrors.ErrHugoBinaryNotFound, err)
	}
	cmd := exec.Command("hugo")
	cmd.Dir = rootDir
	// Let existing runHugoBuild handle stream configuration (stdout/stderr) – reused for minimal churn.
	slog.Debug("BinaryRenderer invoking hugo", "dir", rootDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", herrors.ErrHugoExecutionFailed, err)
	}
	return nil
}

// NoopRenderer performs no rendering; useful in tests or when only scaffolding is desired.
type NoopRenderer struct{}

func (n *NoopRenderer) Execute(rootDir string) error {
	slog.Debug("NoopRenderer skipping render", "dir", rootDir)
	return nil
}

// WithRenderer allows tests or callers to inject a custom renderer.
func (g *Generator) WithRenderer(r Renderer) *Generator {
	if r != nil {
		g.renderer = r
	}
	return g
}
