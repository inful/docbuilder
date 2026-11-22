package httpserver

import "testing"

// A page in the browser of the operator can post a form to the admin
// port: the answer is unreadable, but the side effect happens. The
// requests from another origin are refused.
func TestIsLoopbackOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:8082", true},
		{"http://127.0.0.1:8082", true},
		{"http://[::1]:8082", true},
		{"https://example.com", false},
		{"http://192.168.1.10:8082", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := isLoopbackOrigin(tc.origin); got != tc.want {
			t.Errorf("isLoopbackOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
		}
	}
}
