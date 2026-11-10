package stages

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// StageLayouts writes the layout overrides the build needs.
//
// The Relearn theme provides all necessary layouts via Hugo Modules, so
// there is nothing to write by default. The shared sidebar mode is the one
// exception: it shadows the theme's sidebar partial so the navigation can be
// rendered once and injected in the browser instead of being embedded in
// every page.
func StageLayouts(_ context.Context, bs *models.BuildState) error {
	if bs.Generator.Config().Hugo.Sidebar.IsShared() {
		return bs.Generator.WriteSharedNavFiles()
	}
	return nil
}
