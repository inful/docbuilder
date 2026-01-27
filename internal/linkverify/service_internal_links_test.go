package linkverify

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckInternalLink_SameHostAbsoluteIndexHTML(t *testing.T) {
	publicDir := filepath.Join(t.TempDir(), "public")
	if err := os.MkdirAll(filepath.Join(publicDir, "tags"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "tags", "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write tags/index.html: %v", err)
	}

	page := &PageMetadata{
		HTMLPath:     filepath.Join(publicDir, "tags", "index.html"),
		RenderedPath: filepath.FromSlash("tags/index.html"),
		BaseURL:      "https://www.hbesfb.net/",
	}

	status, err := checkInternalLink(page, "https://www.hbesfb.net/index.html")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status=%d, want %d", status, http.StatusOK)
	}
}

func TestLocalPathForInternalURL_PrettyURLMapsToIndex(t *testing.T) {
	publicDir := filepath.Join(t.TempDir(), "public")
	if err := os.MkdirAll(filepath.Join(publicDir, "tags"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "tags", "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write tags/index.html: %v", err)
	}

	page := &PageMetadata{
		HTMLPath:     filepath.Join(publicDir, "tags", "index.html"),
		RenderedPath: filepath.FromSlash("tags/index.html"),
		BaseURL:      "https://www.hbesfb.net/",
	}

	p, err := localPathForInternalURL(page, "https://www.hbesfb.net/tags/")
	if err != nil {
		t.Fatalf("localPathForInternalURL: %v", err)
	}
	want := filepath.Join(publicDir, "tags", "index.html")
	if p != want {
		t.Fatalf("path=%q, want %q", p, want)
	}
}
