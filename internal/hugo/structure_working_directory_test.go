package hugo

import (
	"os"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The site is moved aside and the staging tree is put in its place
// when the build is done. The output directory must not hold the
// working directory of the process: the process would find its working
// directory gone under its feet.
func TestTheOutputDirectoryMayNotHoldTheWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("the working directory: %v", err)
	}

	g := NewGenerator(&config.Config{}, wd)
	if err := g.beginStaging(); err == nil {
		t.Fatal("the build accepted an output directory that holds the working directory")
	}

	// A directory below the working directory is fine.
	nested := wd + "/site"
	g = NewGenerator(&config.Config{}, nested)
	if err := g.beginStaging(); err != nil {
		t.Fatalf("the build refused an output directory below the working directory: %v", err)
	}
}
