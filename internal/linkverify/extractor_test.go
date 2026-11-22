package linkverify

import (
	"net/url"
	"testing"
)

// The rule decides whether a link is checked against the network. A
// wrong answer sends the verifier after a link that is part of the
// site, or leaves a broken external link unchecked.
func TestIsInternalLink(t *testing.T) {
	base, err := url.Parse("https://docs.example.com/base/")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		linkURL string
		baseURL *url.URL
		want    bool
	}{
		{name: "a mail address", linkURL: "mailto:someone@example.com", baseURL: base, want: true},
		{name: "a telephone number", linkURL: "tel:+15550100", baseURL: base, want: true},
		{name: "a script", linkURL: "javascript:void(0)", baseURL: base, want: true},
		{name: "an anchor", linkURL: "#section", baseURL: base, want: true},
		{name: "a relative path", linkURL: "guide/intro.md", baseURL: base, want: true},
		{name: "a root relative path", linkURL: "/guide/intro.md", baseURL: base, want: true},
		{name: "the same host", linkURL: "https://docs.example.com/guide/intro.md", baseURL: base, want: true},
		{name: "another host", linkURL: "https://other.example.com/guide", baseURL: base, want: false},
		{name: "another host without a base", linkURL: "https://other.example.com/guide", baseURL: nil, want: false},
		{name: "a relative path without a base", linkURL: "guide/intro.md", baseURL: nil, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInternalLink(tt.linkURL, tt.baseURL); got != tt.want {
				t.Errorf("isInternalLink(%q) = %v, want %v", tt.linkURL, got, tt.want)
			}
		})
	}
}
