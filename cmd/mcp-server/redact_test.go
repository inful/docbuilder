package main

import (
	"encoding/json"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestRedactConfig_NilSafe(t *testing.T) {
	got := redactConfig(nil)
	if got == nil {
		t.Fatal("expected empty config for nil input, got nil")
	}
	// Marshalling the empty config must succeed and produce valid JSON.
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("marshal empty config: %v", err)
	}
}

func TestRedactConfig_MasksToken(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{{
			URL:  "https://example.com/foo.git",
			Name: "foo",
			Auth: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "ghp_supersecret",
			},
		}},
	}

	redacted := redactConfig(cfg)
	if redacted == cfg {
		t.Fatal("redactConfig must return a new instance, not the original")
	}
	if got := redacted.Repositories[0].Auth.Token; got != redactedMarker {
		t.Errorf("expected token to be %q, got %q", redactedMarker, got)
	}
	if got := redacted.Repositories[0].URL; got != "https://example.com/foo.git" {
		t.Errorf("URL should be unchanged, got %q", got)
	}
	if got := redacted.Repositories[0].Auth.Type; got != config.AuthTypeToken {
		t.Errorf("auth.type should be unchanged, got %q", got)
	}
	if cfg.Repositories[0].Auth.Token != "ghp_supersecret" {
		t.Error("redactConfig must not mutate the input")
	}
}

func TestRedactConfig_MasksPasswordAndKeyPath(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{{
			URL: "git@github.com:foo/bar.git",
			Auth: &config.AuthConfig{
				Type:     config.AuthTypeBasic,
				Username: "ci-bot",
				Password: "hunter2",
				KeyPath:  "/run/secrets/deploy_key",
			},
		}},
	}
	r := redactConfig(cfg)
	auth := r.Repositories[0].Auth
	if auth.Password != redactedMarker {
		t.Errorf("password: got %q want %q", auth.Password, redactedMarker)
	}
	if auth.KeyPath != redactedMarker {
		t.Errorf("key_path: got %q want %q", auth.KeyPath, redactedMarker)
	}
	if auth.Username != "ci-bot" {
		t.Errorf("username should be unchanged, got %q", auth.Username)
	}
}

func TestRedactConfig_EmptySecretsStayEmpty(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{{
			URL:  "https://example.com/public.git",
			Auth: &config.AuthConfig{Type: config.AuthTypeNone},
		}},
	}
	r := redactConfig(cfg)
	if r.Repositories[0].Auth.Token != "" {
		t.Errorf("empty token should stay empty, got %q", r.Repositories[0].Auth.Token)
	}
}

func TestRedactConfig_JSONOmitsSecrets(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{{
			URL:  "https://example.com/foo.git",
			Auth: &config.AuthConfig{Type: config.AuthTypeToken, Token: "secret"},
		}},
	}
	r := redactConfig(cfg)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "secret") {
		t.Errorf("redacted JSON still contains secret token: %s", string(b))
	}
	if !strings.Contains(string(b), redactedMarker) {
		t.Errorf("redacted JSON missing placeholder: %s", string(b))
	}
}

func TestRedactConfig_NilAuthBlock(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{{
			URL:  "https://example.com/public.git",
			Auth: nil, // public repo, no auth
		}},
	}
	r := redactConfig(cfg)
	if r.Repositories[0].Auth != nil {
		t.Errorf("nil auth should stay nil, got %#v", r.Repositories[0].Auth)
	}
}
