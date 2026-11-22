package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandEnv_BracedFormIsExpanded(t *testing.T) {
	t.Setenv("DBB_TEST_TOKEN", "s3cret")
	got := expandEnv("token: ${DBB_TEST_TOKEN}")
	want := "token: s3cret"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpandEnv_UnsetBracedFormBecomesEmpty(t *testing.T) {
	_ = os.Unsetenv("DBB_TEST_UNSET")
	got := expandEnv("token: ${DBB_TEST_UNSET}")
	want := "token: "
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpandEnv_BareFormIsExpandedWhenSet(t *testing.T) {
	t.Setenv("DBB_TEST_BARE", "value")
	got := expandEnv("token: $DBB_TEST_BARE")
	want := "token: value"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpandEnv_UnsetBareFormIsLeftAlone(t *testing.T) {
	_ = os.Unsetenv("DBB_TEST_MISSING")
	got := expandEnv("token: $DBB_TEST_MISSING")
	want := "token: $DBB_TEST_MISSING"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpandEnv_LiteralDollarIsPreserved(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"price", "title: Cost is $5", "title: Cost is $5"},
		{"password", "desc: p@$ecret", "desc: p@$ecret"},
		{"regex", `pattern: "^\\d+$"`, `pattern: "^\\d+$"`},
		{"lone", "title: $", "title: $"},
		{"double", "title: $$", "title: $$"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandEnv(tc.in); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestExpandEnv_UnterminatedBraceIsLeftAlone(t *testing.T) {
	got := expandEnv("token: ${DBB_TEST_OPEN")
	want := "token: ${DBB_TEST_OPEN"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLoadWithResult_KeepsLiteralDollarInValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := "version: \"2.0\"\nhugo:\n  title: \"Cost is $5\"\n  description: \"p@$ecret\"\n" +
		"repositories:\n  - url: https://example.com/a.git\n    name: a\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Hugo.Title; got != "Cost is $5" {
		t.Errorf("title: got %q want %q", got, "Cost is $5")
	}
	if got := cfg.Hugo.Description; got != "p@$ecret" {
		t.Errorf("description: got %q want %q", got, "p@$ecret")
	}
}
