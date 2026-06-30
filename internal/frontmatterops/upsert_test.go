package frontmatterops

import (
	"errors"
	"strings"
	"testing"
)

// TestUpsertFields_NoFrontmatter_Create covers the create-if-missing path:
// when content has no frontmatter AND createIfMissing is true, the
// helper writes a fresh block with the style newline as separator.
func TestUpsertFields_NoFrontmatter_Create(t *testing.T) {
	const body = "Hello body.\n"

	got, changed, err := UpsertFields(body, true, func(f map[string]any) (bool, error) {
		f["uid"] = "abc-123"
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if !strings.Contains(got, "uid: abc-123") {
		t.Errorf("expected uid in output, got %q", got)
	}
	if !strings.Contains(got, "Hello body.") {
		t.Errorf("expected body preserved, got %q", got)
	}
}

// TestUpsertFields_NoFrontmatter_Skip covers the create-iff-missing=false
// behavior: caller wants modification only; missing frontmatter is a noop.
func TestUpsertFields_NoFrontmatter_Skip(t *testing.T) {
	const body = "Just a body, no frontmatter.\n"

	got, changed, err := UpsertFields(body, false, func(f map[string]any) (bool, error) {
		f["uid"] = "abc-123"
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false (no frontmatter to mutate)")
	}
	if got != body {
		t.Fatalf("expected unchanged content %q, got %q", body, got)
	}
}

// TestUpsertFields_Existing_Changed covers the mutation path: existing
// frontmatter, mutate returns true, output round-trips with the mutation.
func TestUpsertFields_Existing_Changed(t *testing.T) {
	const src = "---\nuid: old\n---\nbody\n"

	got, changed, err := UpsertFields(src, true, func(f map[string]any) (bool, error) {
		if f["uid"] != "old" {
			return false, errors.New("expected old uid")
		}
		f["uid"] = "new"
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if !strings.Contains(got, "uid: new") {
		t.Errorf("expected new uid, got %q", got)
	}
	if !strings.Contains(got, "body") {
		t.Errorf("expected body preserved, got %q", got)
	}
}

// TestUpsertFields_Existing_NoChange covers the short-circuit path:
// mutate returns false (or err), the helper bails out with the original
// content and (false, nil/err).
func TestUpsertFields_Existing_NoChange(t *testing.T) {
	const src = "---\nuid: present\n---\nbody\n"

	t.Run("mutate says no change", func(t *testing.T) {
		got, changed, err := UpsertFields(src, true, func(f map[string]any) (bool, error) {
			return false, nil // I made no changes
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if changed {
			t.Errorf("expected changed=false")
		}
		if got != src {
			t.Errorf("expected unchanged content, got %q", got)
		}
	})

	t.Run("mutate returns error", func(t *testing.T) {
		wantErr := errors.New("boom")
		got, changed, err := UpsertFields(src, true, func(f map[string]any) (bool, error) {
			return false, wantErr
		})
		if err == nil {
			t.Fatalf("expected err")
		}
		if changed {
			t.Errorf("expected changed=false on error")
		}
		if got != src {
			t.Errorf("expected unchanged content on error, got %q", got)
		}
	})
}

// TestUpsertFields_ReadError covers the malformed-frontmatter case: Read
// returns an error; UpsertFields surfaces it and bails out.
func TestUpsertFields_ReadError(t *testing.T) {
	// Unterminated YAML frontmatter (no closing ---).
	const src = "---\nuid: oops\n"

	got, changed, err := UpsertFields(src, true, func(f map[string]any) (bool, error) {
		t.Fatalf("mutate must not run when Read fails")
		return true, nil
	})
	if err == nil {
		t.Fatalf("expected read error, got nil")
	}
	if changed {
		t.Errorf("expected changed=false on read error")
	}
	if got != src {
		t.Errorf("expected unchanged content on read error, got %q", got)
	}
}

// TestUpsertFields_BodyPreserved covers that body content is not
// mangled when adding frontmatter: the style.Newline is prepended
// exactly once if not already present.
func TestUpsertFields_BodyPreserved(t *testing.T) {
	t.Run("body without leading newline", func(t *testing.T) {
		const body = "First paragraph.\n"
		got, _, err := UpsertFields(body, true, func(f map[string]any) (bool, error) {
			f["uid"] = "x"
			return true, nil
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// Frontmatter block + body's "First paragraph." must still appear
		// verbatim. The helper inserts a single leading newline after the
		// closing delimiter if the body didn't have one, so the body
		// itself is unchanged.
		if !strings.Contains(got, "First paragraph.") {
			t.Errorf("body not preserved: %q", got)
		}
	})

	t.Run("body with leading newline", func(t *testing.T) {
		const body = "\nleading-newline body\n"
		got, _, err := UpsertFields(body, true, func(f map[string]any) (bool, error) {
			f["uid"] = "x"
			return true, nil
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.Contains(got, "leading-newline body") {
			t.Errorf("body not preserved: %q", got)
		}
	})
}
