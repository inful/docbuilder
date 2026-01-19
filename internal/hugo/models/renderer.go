package models

import (
	"context"
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
	Execute(ctx context.Context, rootDir string) error
}
