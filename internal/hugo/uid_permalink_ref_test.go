package hugo

import "testing"

func TestInjectUIDPermalink_AppendsWhenUIDAndAliasMatch(t *testing.T) {
	in := "---\nuid: abc123\naliases:\n  - /_uid/abc123/\n---\n\n# Title\n\nBody\n"
	baseURL := "https://example.com/docs/"
	out, changed := injectUIDPermalink(in, baseURL)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	want := "{{% badge style=\"note\" title=\"permalink\" %}}`https://example.com/docs/_uid/abc123/`{{% /badge %}}"
	if out[len(out)-len(want)-1:len(out)-1] != want {
		t.Fatalf("expected permalink line at end, got: %q", out)
	}
}

func TestInjectUIDPermalink_NoChangeWhenAliasMissing(t *testing.T) {
	in := "---\nuid: abc123\n---\n\n# Title\n"
	out, changed := injectUIDPermalink(in, "http://localhost")
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalink_NoChangeWhenAliasDoesNotMatchUID(t *testing.T) {
	in := "---\nuid: abc123\naliases:\n  - /_uid/zzz/\n---\n\n# Title\n"
	out, changed := injectUIDPermalink(in, "http://localhost")
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalink_Idempotent(t *testing.T) {
	in := "---\nuid: abc123\naliases: [\"/_uid/abc123/\"]\n---\n\nBody\n\n{{% badge style=\"note\" title=\"permalink\" %}}`https://example.com/_uid/abc123/`{{% /badge %}}\n"
	out, changed := injectUIDPermalink(in, "https://example.com")
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalink_NoOpWhenOldRefFormatAlreadyPresent(t *testing.T) {
	in := "---\nuid: abc123\naliases: [\"/_uid/abc123/\"]\n---\n\nBody\n\n[Permalink]({{% ref \"/_uid/abc123/\" %}})\n"
	out, changed := injectUIDPermalink(in, "http://localhost")
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalink_NoOpWhenOldPlainFormatAlreadyPresent(t *testing.T) {
	in := "---\nuid: abc123\naliases: [\"/_uid/abc123/\"]\n---\n\nBody\n\n[Permalink](/_uid/abc123/)\n"
	out, changed := injectUIDPermalink(in, "http://localhost")
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}

func TestInjectUIDPermalink_NoOpWhenBadgeWithoutBaseURLAlreadyPresent(t *testing.T) {
	in := "---\nuid: abc123\naliases: [\"/_uid/abc123/\"]\n---\n\nBody\n\n{{% badge style=\"note\" title=\"permalink\" %}}`/_uid/abc123/`{{% /badge %}}\n"
	out, changed := injectUIDPermalink(in, "https://example.com")
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != in {
		t.Fatalf("expected content unchanged")
	}
}
