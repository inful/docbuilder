package httpserver

import (
	"log/slog"
	"testing"
)

// The failure of the edit handler says what failed.
//
// The message alone ("the IPC socket was not found") does not say which
// one, and the page in the browser is what the developer sees.
func TestTheFailureOfTheEditHandlerCarriesItsDetails(t *testing.T) {
	details := formatEditErrorDetails([]any{
		slog.String("path", "/docs/guide.md"),
		slog.String("hint", "Ensure VS Code is running"),
	})

	if details == "" {
		t.Fatal("the details are empty")
	}
	if !contains(details, "/docs/guide.md") {
		t.Fatalf("the details do not carry the path: %q", details)
	}
	if !contains(details, "Ensure VS Code is running") {
		t.Fatalf("the details do not carry the hint: %q", details)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
