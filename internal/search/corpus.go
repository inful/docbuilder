package search

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

// Collect reads the documents of a built site.
//
// The site directory holds two trees that describe the same pages: the
// transformed markdown in "content" and the rendered pages in "public".
// The markdown carries the text, the front matter and the taxonomy of a
// page; the rendered tree carries the address the page is served at.
//
// Only pages that were actually rendered are collected: a page that is
// not in "public" cannot be reached, so it must not show up in a
// search result.
//
// The taxonomy pages Hugo generates ("/tags/<tag>/") are not in the
// content tree and are therefore not collected. They repeat the text of
// every page that carries the tag, and in a site with many tags they
// were more than half of what the browser-side search indexed.
func Collect(siteDir string) ([]Document, error) {
	contentDir := filepath.Join(siteDir, "content")
	publicDir := filepath.Join(siteDir, "public")

	rendered, err := renderedURLs(publicDir)
	if err != nil {
		return nil, err
	}

	var docs []Document
	walkErr := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		rel, relErr := filepath.Rel(contentDir, path)
		if relErr != nil {
			return relErr
		}
		doc, skip, docErr := documentFor(path, rel, rendered)
		if docErr != nil {
			// One page that cannot be read must not empty the whole
			// index. The page is left out and the build carries on.
			slog.Warn("Skipping a page that cannot be indexed",
				logfields.Path(path), logfields.Error(docErr))
			return nil
		}
		if skip {
			return nil
		}
		docs = append(docs, *doc)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("search: read the content tree: %w", walkErr)
	}

	sort.Slice(docs, func(i, j int) bool { return docs[i].URL < docs[j].URL })
	return docs, nil
}

// renderedURLs maps the path of a rendered file to the address it is
// served at. A directory index is served at the directory itself, so
// "adr/adr-001/index.html" is served at "/adr/adr-001/".
func renderedURLs(publicDir string) (map[string]string, error) {
	urls := make(map[string]string)
	err := filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// Nothing was rendered. The site has no pages yet.
				return filepath.SkipDir
			}
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".html") {
			return nil
		}
		rel, relErr := filepath.Rel(publicDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		switch {
		case rel == "index.html":
			urls[rel] = "/"
		case strings.HasSuffix(rel, "/index.html"):
			urls[rel] = "/" + strings.TrimSuffix(rel, "index.html")
		default:
			urls[rel] = "/" + rel
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search: read the rendered tree: %w", err)
	}
	return urls, nil
}

// urlForContentPath returns the address of the page the content file
// was rendered at, or false when it was not rendered.
//
// A page can ask for an address of its own with "url" in the front
// matter: it is then rendered there, and nowhere else.
func urlForContentPath(rel string, fields map[string]any, rendered map[string]string) (string, bool) {
	if own, ok := fields["url"].(string); ok && own != "" {
		url, found := renderedURLForOwn(own, rendered)
		if found {
			return url, true
		}
	}

	rel = filepath.ToSlash(rel)
	base := strings.TrimSuffix(rel, ".md")
	dir := path.Dir(base)

	candidates := make([]string, 0, 2)
	candidates = append(candidates, base+"/index.html")
	if base == "_index" {
		candidates = []string{"index.html"}
	} else if path.Base(base) == "_index" {
		candidates = []string{path.Join(dir, "index.html")}
	}
	candidates = append(candidates, base+".html")

	for _, candidate := range candidates {
		if url, ok := rendered[candidate]; ok {
			return url, true
		}
	}
	return "", false
}

// renderedURLForOwn returns the address of a page that asked for an
// address of its own.
//
// The address is the one the page is served at, so it comes from the
// rendered tree: the page is looked up under the name it was rendered
// with.
func renderedURLForOwn(own string, rendered map[string]string) (string, bool) {
	own = strings.TrimSuffix(own, "/")
	own = strings.TrimPrefix(own, "/")
	if own == "" {
		own = ""
	}

	candidates := []string{own + "/index.html", own + ".html"}
	if own == "" {
		candidates = []string{"index.html"}
	}
	for _, candidate := range candidates {
		if url, ok := rendered[candidate]; ok {
			return url, true
		}
	}
	return "", false
}

// documentFor reads one content file into a document. It reports skip
// when the page asks not to be searched.
//
// A page that was not rendered is left out: it cannot be reached, so it
// must not turn up in a search result.
func documentFor(path, rel string, rendered map[string]string) (*Document, bool, error) {
	parsed, err := docmodel.ParseFile(path, docmodel.Options{})
	if err != nil {
		return nil, false, fmt.Errorf("search: read %s: %w", path, err)
	}

	fields, err := parsed.FrontmatterFields()
	if err != nil {
		return nil, false, fmt.Errorf("search: read the front matter of %s: %w", path, err)
	}
	if excluded, ok := fields["exclude_search"].(bool); ok && excluded {
		return nil, true, nil
	}

	url, ok := urlForContentPath(rel, fields, rendered)
	if !ok {
		return nil, true, nil
	}

	doc := &Document{
		URL:        url,
		Title:      stringField(fields, "title"),
		Categories: listField(fields, "categories"),
		Tags:       listField(fields, "tags"),
		// The text of the page, not the markup around it: the reader
		// searches for words and reads excerpts.
		Body: markdown.PlainText(parsed.Body()),
	}
	if doc.Title == "" {
		doc.Title = defaultTitle(url)
	}
	return doc, false, nil
}

// stringField reads a string field from the front matter.
func stringField(fields map[string]any, name string) string {
	value, _ := fields[name].(string)
	return value
}

// listField reads a taxonomy field from the front matter. It holds a
// list of names, or a single name.
func listField(fields map[string]any, name string) string {
	switch value := fields[name].(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case []string:
		return strings.Join(value, " ")
	default:
		return ""
	}
}

// defaultTitle is the title of a page without one: the last part of
// its address, with the dashes turned into spaces.
func defaultTitle(url string) string {
	trimmed := strings.Trim(url, "/")
	if trimmed == "" {
		return "Documentation"
	}
	parts := strings.Split(trimmed, "/")
	last := strings.ReplaceAll(parts[len(parts)-1], "-", " ")
	// The first letter is upper-cased. It is a letter, not a byte: a
	// byte would cut a multi-byte letter in half.
	runes := []rune(last)
	return string(unicode.ToUpper(runes[0])) + string(runes[1:])
}
