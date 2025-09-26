package repository

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestRepositoryFilterBasic(t *testing.T) {
	f, err := NewFilter([]string{"api-*", "core"}, []string{"*-deprecated"})
	if err != nil {
		t.Fatalf("filter creation failed: %v", err)
	}
	cases := []struct {
		name   string
		expect bool
	}{
		{"api-users", true},
		{"api-deprecated", false},
		{"core", true},
		{"random", false},
	}
	for _, c := range cases {
		repo := config.Repository{Name: c.name}
		ok, _ := f.Include(repo)
		if ok != c.expect {
			t.Errorf("repo %s expected %v got %v", c.name, c.expect, ok)
		}
	}
}

func TestRepositoryFilterExcludePrecedence(t *testing.T) {
	f, _ := NewFilter([]string{"*"}, []string{"secret-*"})
	repo := config.Repository{Name: "secret-config"}
	if ok, reason := f.Include(repo); ok || reason == "" {
		t.Fatalf("expected exclusion precedence, got ok=%v reason=%s", ok, reason)
	}
}

func TestRepositoryFilterEmptyIncludes(t *testing.T) {
	f, _ := NewFilter(nil, []string{"skip-*"})
	if ok, _ := f.Include(config.Repository{Name: "alpha"}); !ok {
		t.Fatal("expected include by default")
	}
	if ok, reason := f.Include(config.Repository{Name: "skip-me"}); ok || reason == "" {
		t.Fatalf("expected excluded skip-me reason got ok=%v reason=%s", ok, reason)
	}
}
