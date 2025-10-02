package transforms

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoReflectUsage ensures the transforms package does not import the reflect package,
// enforcing the architectural guideline of reflection-free transformers.
func TestNoReflectUsage(t *testing.T) {
	// Discover current package directory using runtime caller.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("runtime caller unavailable")
	}
	dir := filepath.Dir(file)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse dir: %v", err)
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == "reflect" {
					t.Fatalf("reflect import found in transforms package (file %s) which is disallowed", fset.File(f.Pos()).Name())
				}
			}
		}
	}
}
