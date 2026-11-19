package httpserver

import "testing"

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
		{name: "a stylesheet", path: "/css/site.css", want: "public, max-age=31536000, immutable"},
		{name: "a script", path: "/js/site.js", want: "public, max-age=31536000, immutable"},
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
