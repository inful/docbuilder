package httpserver

import (
	"strings"
	"testing"
)

// The cache header decides what a reader's browser does with a page.
// A wrong one either serves stale content for a year or re-downloads
// everything on every visit.
func TestDetermineCacheControl(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "the home page", path: "/", want: cacheControlNoCache},
		{name: "a page", path: "/docs/guide/", want: cacheControlNoCache},
		{name: "an html file", path: "/docs/guide.html", want: cacheControlNoCache},
		{name: "a fingerprinted stylesheet", path: "/css/site.min.0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.css", want: "public, max-age=31536000, immutable"},
		{name: "a fingerprinted script", path: "/js/site.min.0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.js", want: "public, max-age=31536000, immutable"},
		{name: "an unfingerprinted stylesheet", path: "/css/prism.css", want: cacheControlFiveMinute},
		{name: "an unfingerprinted script", path: "/js/prism.js", want: cacheControlFiveMinute},
		{name: "a font", path: "/fonts/site.woff2", want: "public, max-age=31536000, immutable"},
		{name: "an image", path: "/img/logo.png", want: cacheControlWeek},
		{name: "a download", path: "/files/manual.pdf", want: cacheControlDay},
		{name: "a json file", path: "/api/taxonomies.json", want: cacheControlFiveMinute},
		{name: "a search index", path: "/search.json", want: ""},
		{name: "a feed", path: "/index.xml", want: cacheControlHour},
		{name: "something else", path: "/data/file.bin", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineCacheControl(tt.path); got != tt.want {
				t.Errorf("determineCacheControl(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// Only a fingerprinted name may be cached for a year: its URL changes
// when the content does. Everything else has to be revalidated.
func TestIsContentAddressed(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef"

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "a fingerprinted stylesheet", path: "/css/site.min." + hash + ".css", want: true},
		{name: "a fingerprinted script", path: "/js/site.min." + hash + ".js", want: true},
		{name: "a hash in upper case", path: "/css/site.min." + strings.ToUpper(hash) + ".css", want: true},
		{name: "a hash in a directory", path: "/css/" + hash + "/site.css", want: false},
		{name: "a stable stylesheet", path: "/css/prism.css", want: false},
		{name: "a short hash", path: "/css/site.min.0123456789abcdef.css", want: false},
		{name: "a hash that is not hexadecimal", path: "/css/site.min.0123456789abcdefg123456789abcdef.css", want: false},
		{name: "a hash that is not the last segment", path: "/css/site.min." + hash + ".css.map", want: false},
		{name: "a name without an extension", path: "/css/site.min." + hash, want: false},
		{name: "a name without a hash", path: "/css/site.css", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContentAddressed(tt.path); got != tt.want {
				t.Errorf("isContentAddressed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
