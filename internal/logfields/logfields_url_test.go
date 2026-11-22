package logfields

import "testing"

// A repository URL is the one field where an operator puts a token. The
// credentials must not reach the logs.
func TestURLHidesTheCredentials(t *testing.T) {
	got := URL("https://user:token@example.com/org/repo.git")
	value := got.Value.String()
	if value != "https://example.com/org/repo.git" {
		t.Fatalf("the url is %q", value)
	}
}

// A URL without credentials stays as it is.
func TestURLKeepsAURLWithoutCredentials(t *testing.T) {
	got := URL("https://example.com/org/repo.git")
	if got.Value.String() != "https://example.com/org/repo.git" {
		t.Fatalf("the url is %q", got.Value.String())
	}
}
