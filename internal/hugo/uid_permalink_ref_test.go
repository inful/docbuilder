package hugo

import "testing"

func TestInjectUIDPermalinkRefShortcode_AppendsWhenUIDAndAliasMatch(t *testing.T) {
	in := "---\nuid: abc123\naliases:\n  - /_uid/abc123/\n---\n\n# Title\n\nBody\n"
	out, changed := injectUIDPermalinkRefShortcode(in)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	want := "[Permalink](/_uid/abc123/)"
	if out[len(out)-len(want)-1:len(out)-1] != want {
		t.Fatalf("expected permalink line at end, got: %q", out)
	}
}

func TestInjectUIDPermalinkRefShortcode_NoChangeWhenAliasMissing(t *testing.T) {
	in := "---\nuid: abc123\n---\n\n# Title\n"
	out, changed := injectUIDPermalinkRefShortcode(in)
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalinkRefShortcode_NoChangeWhenAliasDoesNotMatchUID(t *testing.T) {
	in := "---\nuid: abc123\naliases:\n  - /_uid/zzz/\n---\n\n# Title\n"
	out, changed := injectUIDPermalinkRefShortcode(in)
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalinkRefShortcode_Idempotent(t *testing.T) {
	in := "---\nuid: abc123\naliases: [\"/_uid/abc123/\"]\n---\n\nBody\n\n[Permalink](/_uid/abc123/)\n"
	out, changed := injectUIDPermalinkRefShortcode(in)
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalinkRefShortcode_NoOpWhenOldRefFormatAlreadyPresent(t *testing.T) {
	in := "---\nuid: abc123\naliases: [\"/_uid/abc123/\"]\n---\n\nBody\n\n[Permalink]({{% ref \"/_uid/abc123/\" %}})\n"
	out, changed := injectUIDPermalinkRefShortcode(in)
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}
