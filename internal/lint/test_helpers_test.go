package lint

import (
	"os"
	"testing"
)

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("skipping permission-dependent test when running as root")
	}
}
