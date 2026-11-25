package commands

import (
	"strings"
	"testing"
)

// A typo in the render mode must not publish a site without HTML: the
// mode became "auto", and in "auto" the build does not run hugo. The
// build said it was fine, and the site held the scaffold only.
func TestTheBuildFailsOnATypoInTheRenderMode(t *testing.T) {
	t.Chdir(t.TempDir())

	b := &BuildCmd{DocsDir: "./docs", RenderMode: "alwais"}
	root := &CLI{}

	err := b.Run(&Global{}, root)
	if err == nil {
		t.Fatal("the build accepted a render mode that it does not know")
	}
	if !strings.Contains(err.Error(), "alwais") {
		t.Fatalf("the error does not name the render mode: %v", err)
	}
}

// The render mode of the flag is written in any case: the configuration
// file accepts "AlWaYs".
func TestTheRenderModeOfTheFlagIsAcceptedInAnyCase(t *testing.T) {
	t.Chdir(t.TempDir())

	b := &BuildCmd{DocsDir: "./docs", RenderMode: "AlWaYs"}
	root := &CLI{}

	// The build of the local mode runs; the render mode must not stop
	// it.
	err := b.Run(&Global{}, root)
	if err != nil && strings.Contains(err.Error(), "render mode") {
		t.Fatalf("the render mode was refused: %v", err)
	}
}
