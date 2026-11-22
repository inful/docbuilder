package search

import (
	"testing"
)

// A page without a title of its own gets one from its address. The
// first letter is upper-cased, and it must survive a multi-byte
// letter: a byte would cut it in half.
func TestDefaultTitle(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"a plain name", "/install/", "Install"},
		{"dashes become spaces", "/getting-started/", "Getting started"},
		{"the last part of the address", "/how-to/install/", "Install"},
		{"a multi-byte letter", "/übersicht/", "Übersicht"},
		{"another multi-byte letter", "/Ясно/", "Ясно"},
		{"the root", "/", "Documentation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultTitle(tc.url); got != tc.want {
				t.Fatalf("defaultTitle(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
