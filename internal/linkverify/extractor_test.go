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
		{name: "an inline document", linkURL: "data:text/html,<script>", baseURL: base, want: true},
		{name: "a vbscript", linkURL: "vbscript:msgbox(1)", baseURL: base, want: true},
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

// A link with a scheme the browser runs as code is not a page the
// verifier can fetch, so it never goes to the network.
func TestShouldVerifyLink(t *testing.T) {
	tests := []struct {
		name    string
		linkURL string
		want    bool
	}{
		{name: "a page", linkURL: "https://example.com/guide", want: true},
		{name: "a relative page", linkURL: "guide/intro.md", want: true},
		{name: "an anchor", linkURL: "#section", want: false},
		{name: "a mail address", linkURL: "mailto:someone@example.com", want: false},
		{name: "a telephone number", linkURL: "tel:+15550100", want: false},
		{name: "a script", linkURL: "javascript:void(0)", want: false},
		{name: "an inline document", linkURL: "data:text/html,<script>", want: false},
		{name: "a vbscript", linkURL: "vbscript:msgbox(1)", want: false},
		{name: "an empty link", linkURL: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldVerifyLink(&Link{URL: tt.linkURL}); got != tt.want {
				t.Errorf("ShouldVerifyLink(%q) = %v, want %v", tt.linkURL, got, tt.want)
			}
		})
	}
}
