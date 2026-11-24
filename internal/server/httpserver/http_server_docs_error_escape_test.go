package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The message of a build failure holds the name of the file that
// failed and the words of Hugo. It goes into the page as text, not as
// markup: a file whose name holds a script would run it in the
// browser of the reader.
func TestTheBuildErrorPageEscapesTheError(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Output: config.OutputConfig{Directory: tmpDir},
	}

	srv := New(cfg, testRuntime{}, Options{BuildStatus: testBuildStatus{
		hasError: true,
		err:      errBuildWithMarkup(),
	}})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	root := srv.resolveDocsRoot()
	srv.handleStatusPage(rec, req, root)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)
	if strings.Contains(bodyStr, "<script>alert(1)</script>") {
		t.Fatalf("the error page holds the raw markup: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "&lt;script&gt;") {
		t.Fatalf("the error page does not hold the escaped error: %s", bodyStr)
	}
}

// markupError is a build failure whose text holds markup.
type markupError struct{}

func (markupError) Error() string {
	return `Error: error building site: process: readAndProcessContent: "/content/repos/x/<script>alert(1)</script>.md:3:1": something failed`
}

func errBuildWithMarkup() error {
	return markupError{}
}
