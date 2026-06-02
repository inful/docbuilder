package foundation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCaseSensitiveFilesystem_AgreesWithProbe(t *testing.T) {
	dir := t.TempDir()
	got := IsCaseSensitiveFilesystem(dir)

	// Manually reproduce the probe to double-check the helper's answer.
	lower := filepath.Join(dir, "lowercase")
	upper := filepath.Join(dir, "LowerCase")
	if err := os.WriteFile(lower, []byte("a"), 0o600); err != nil {
		t.Fatalf("write lower: %v", err)
	}
	defer func() { _ = os.Remove(lower) }()
	if err := os.WriteFile(upper, []byte("b"), 0o600); err != nil {
		t.Fatalf("write upper: %v", err)
	}
	defer func() { _ = os.Remove(upper) }()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	hasLower, hasUpper := false, false
	for _, e := range entries {
		switch e.Name() {
		case "lowercase":
			hasLower = true
		case "LowerCase":
			hasUpper = true
		}
	}
	want := hasLower && hasUpper

	if got != want {
		t.Errorf("IsCaseSensitiveFilesystem(%q)=%v but manual probe=%v", dir, got, want)
	}
}

func TestIsCaseSensitiveFilesystem_EmptyDirUsesProbe(t *testing.T) {
	// Calling with an empty dir must not panic and must return a bool.
	_ = IsCaseSensitiveFilesystem("")
}
