// Package search provides the server-side full-text index over the
// rendered documentation.
//
// The index is a SQLite FTS5 table. SQLite is already a dependency of
// this binary (the event store uses it), and FTS5 ships with it, so a
// full-text index with BM25 ranking, stemming and highlighted excerpts
// costs no new module.
//
// The index lives in the daemon's state directory and is owned by the
// daemon: the build feeds it the pages it rendered, and the docs server
// queries it. Nothing is sent to the visitor except the result of the
// query.
package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// Document is one page of the documentation as it is indexed.
type Document struct {
	// URL is the path of the page on the site, starting with a slash.
	URL string
	// Title is the page title.
	Title string
	// Categories are the taxonomy categories of the page.
	Categories string
	// Tags are the taxonomy tags of the page.
	Tags string
	// Body is the text of the page.
	Body string
}

// Result is one hit of a search.
type Result struct {
	URL     string  `json:"url"`
	Title   string  `json:"title"`
	Excerpt string  `json:"excerpt"`
	Score   float64 `json:"score"`
}

// searchStatement is the query the index is searched with.
//
// The weights of the bm25() function are per column: a hit in the
// title counts more than a hit in the body. The URL is not searched, so
// its weight is zero. The weights have to be literals: FTS5 reads the
// arguments of an auxiliary function when the statement is prepared,
// and a bound parameter is not a value yet.
const searchStatement = `SELECT url, title, snippet(pages, 4, '', '', '…', 20), ` +
	`bm25(pages, 0.0, 5.0, 2.0, 2.0, 1.0) FROM pages ` +
	`WHERE pages MATCH ? ORDER BY rank LIMIT ?`

// Index is the full-text index over the documentation.
type Index struct {
	db *sql.DB
}

// Open opens the index in the given directory, creating it when it does
// not exist yet.
func Open(dir string) (*Index, error) {
	if dir == "" {
		return nil, errors.New("search: index directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("search: create index directory: %w", err)
	}

	// The pragmas go into the DSN, not into a statement. A statement
	// runs on one connection of the pool; the pool opens more when the
	// load grows, and those connections would have the default of no
	// wait at all. The DSN applies the pragmas to every connection the
	// pool opens.
	//
	// The daemon serves search requests while a build writes to the
	// index, so readers must not block the writer and vice versa.
	//
	// The transaction starts as "immediate": a transaction that starts
	// as "deferred" takes the write lock on its first write, and SQLite
	// does not wait for the lock on that step - it fails at once. Two
	// builds finishing at the same time would then break each other.
	path := filepath.Join(dir, "search.db")
	dsn := "file:" + path + "?" + url.Values{
		"_pragma": {
			"busy_timeout(5000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
		},
		"_txlock": {"immediate"},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("search: open index: %w", err)
	}

	idx := &Index{db: db}
	if err := idx.createSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return idx, nil
}

// createSchema creates the FTS5 table when it is missing.
func (i *Index) createSchema(ctx context.Context) error {
	const schema = `
CREATE VIRTUAL TABLE IF NOT EXISTS pages USING fts5(
    url UNINDEXED,
    title,
    categories,
    tags,
    body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);
`
	if _, err := i.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("search: create schema: %w", err)
	}
	return nil
}

// Close closes the index.
func (i *Index) Close() error {
	if i == nil || i.db == nil {
		return nil
	}
	return i.db.Close()
}

// Upsert replaces the indexed pages with the given documents.
//
// The whole set is replaced in one transaction: the index always holds
// exactly the pages of the last build. A page that is gone from the
// site is gone from the index with it.
func (i *Index) Upsert(ctx context.Context, docs []Document) error {
	if i == nil || i.db == nil {
		return errors.New("search: index is not open")
	}

	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("search: begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, deleteErr := tx.ExecContext(ctx, `DELETE FROM pages`); deleteErr != nil {
		return fmt.Errorf("search: clear index: %w", deleteErr)
	}

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO pages (url, title, categories, tags, body) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("search: prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, doc := range docs {
		if doc.URL == "" {
			continue
		}
		if _, insertErr := stmt.ExecContext(ctx, doc.URL, doc.Title, doc.Categories, doc.Tags, doc.Body); insertErr != nil {
			return fmt.Errorf("search: index %s: %w", doc.URL, insertErr)
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("search: commit: %w", commitErr)
	}
	return nil
}

// Count returns how many pages the index holds.
func (i *Index) Count(ctx context.Context) (int, error) {
	var n int
	if err := i.db.QueryRowContext(ctx, `SELECT count(*) FROM pages`).Scan(&n); err != nil {
		return 0, fmt.Errorf("search: count: %w", err)
	}
	return n, nil
}

// Search returns the pages that match the query, best match first.
//
// The query is read as it is written, so the query language of the
// index works: "AND", "OR" and "NOT" combine the terms, a phrase in
// quotes has to appear as it is written, and "title:", "body:",
// "tags:" and "categories:" restrict a term to a field. The words are
// stemmed the same way the index is, so "deploying" finds "deploy",
// and the last term of a query may be a prefix, so "deploy kube"
// finds "deployment/kubernetes" while the reader is still typing.
//
// A query the index cannot read - a half-typed one, most of the time -
// is searched the forgiving way: every word of the query has to
// appear somewhere in the page.
func (i *Index) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if i == nil || i.db == nil {
		return nil, errors.New("search: index is not open")
	}
	if limit <= 0 {
		limit = 10
	}

	// The query is cut short before it reaches either path. Both paths
	// build one term per word, so a long query would cost the server
	// seconds of work for one request.
	query = cutQuery(query)

	// The forgiving query holds every word of the query. When it is
	// empty the query has no word to search for at all.
	forgiving := buildMatch(query)
	if forgiving == "" {
		return []Result{}, nil
	}

	// The query as it is written comes first, so the operators work.
	// The forgiving query is the fallback for a query the index cannot
	// read.
	matches := make([]string, 0, 2)
	if strict := strictMatch(query); strict != "" {
		matches = append(matches, strict)
	}
	matches = append(matches, forgiving)

	var lastErr error
	for _, match := range matches {
		results, err := i.query(ctx, match, limit)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return []Result{}, nil
}

// query runs one match expression and returns the pages that match it.
func (i *Index) query(ctx context.Context, match string, limit int) ([]Result, error) {
	rows, err := i.db.QueryContext(ctx, searchStatement, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search: query %q: %w", match, err)
	}
	defer func() { _ = rows.Close() }()

	// An empty list, not a nil one: the answer of the endpoint holds a
	// list of results even when there are none.
	results := []Result{}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.URL, &r.Title, &r.Excerpt, &r.Score); err != nil {
			return nil, fmt.Errorf("search: read result: %w", err)
		}
		// bm25 returns a negative number, and the better the match the
		// more negative it is. Report it the other way round, so that a
		// bigger score is a better match.
		r.Score = -r.Score
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search: read results: %w", err)
	}
	return results, nil
}

// operators are the words the query language of the index uses to
// combine the terms of a query. They are the words of the engine: in
// upper case. A lower case "and" is a word the pages have to hold.
var operators = map[string]bool{
	"AND":  true,
	"OR":   true,
	"NOT":  true,
	"NEAR": true,
}

// maxQueryLength is the longest query the index reads. A longer one
// is cut short: a search box is for words, not for programs.
const maxQueryLength = 256

// cutQuery cuts a query short: a search box is for words, not for
// programs.
//
// The cut happens on a rune boundary, so a multi-byte letter is not
// cut in half.
func cutQuery(query string) string {
	query = strings.TrimSpace(query)
	if len(query) > maxQueryLength {
		query = query[:maxQueryLength]
		// The cut can land in the middle of a letter. The half that
		// is left is not a word, so it goes.
		if !utf8.ValidString(query) {
			query = strings.ToValidUTF8(query, "")
		}
	}
	return strings.TrimSpace(query)
}

// strictMatch turns a query into an expression the index reads as it
// is written.
//
// The query is expected to be cut short already (see cutQuery).
//
// The last term of the query may be a prefix, so "deploy kube" finds
// "deployment/kubernetes" while the reader is still typing it.
func strictMatch(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	return withTrailingPrefix(quoteFieldValues(query))
}

// withTrailingPrefix adds the prefix wildcard to the last term of the
// query, when that term is a word outside a phrase.
//
// A term that already ends in a wildcard keeps it, and a query that
// ends in an operator is left alone: the operator is not a term, and
// the query is not finished.
func withTrailingPrefix(query string) string {
	inPhrase := false
	wordStart, wordEnd := -1, -1
	for i, r := range query {
		switch {
		case r == '"':
			inPhrase = !inPhrase
			wordStart, wordEnd = -1, -1
		case inPhrase:
			// A word inside a phrase is part of the phrase.
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if wordStart < 0 {
				wordStart = i
			}
			wordEnd = i + utf8.RuneLen(r)
		default:
			wordStart, wordEnd = -1, -1
		}
	}
	if wordStart < 0 {
		return query
	}
	if operators[query[wordStart:wordEnd]] {
		return query
	}
	return query[:wordEnd] + "*" + query[wordEnd:]
}

// quoteFieldValues quotes the value of a field filter when the value
// holds a character the index would read as syntax.
//
// "categories:how-to" asks for the category "how-to", but the index
// reads the hyphen as the end of the field name: it looks for a field
// named "to" and finds none, so the query finds nothing at all. The
// value is quoted, so it means what the reader wrote: 'categories:"how-to"'.
func quoteFieldValues(query string) string {
	var out strings.Builder
	runes := []rune(query)
	for i := 0; i < len(runes); {
		// A field filter is a name, a colon and a value.
		nameEnd := i
		for nameEnd < len(runes) && isNameRune(runes[nameEnd]) {
			nameEnd++
		}
		if nameEnd == i || nameEnd >= len(runes) || runes[nameEnd] != ':' {
			// Not a field filter. Copy the rune and move on.
			out.WriteRune(runes[i])
			i++
			continue
		}

		// The name of the field, then the colon.
		out.WriteString(string(runes[i:nameEnd]))
		out.WriteRune(':')
		i = nameEnd + 1

		// The value of the field runs to the next space.
		valueEnd := i
		for valueEnd < len(runes) && !unicode.IsSpace(runes[valueEnd]) {
			valueEnd++
		}
		value := string(runes[i:valueEnd])
		i = valueEnd

		// A value that is already quoted is left alone, and so is one
		// the index reads as it is: a plain word.
		if value == "" || value[0] == '"' || isPlainWord(value) {
			out.WriteString(value)
			continue
		}

		// The value is quoted. A quote inside it is doubled, the way
		// the index reads it.
		out.WriteByte('"')
		out.WriteString(strings.ReplaceAll(value, `"`, `""`))
		out.WriteByte('"')
	}
	return out.String()
}

// isNameRune reports whether the rune can be part of a field name.
func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// isPlainWord reports whether the value is a word the index reads as
// it is: letters, digits and the underscore.
func isPlainWord(value string) bool {
	for _, r := range value {
		if !isNameRune(r) {
			return false
		}
	}
	return value != ""
}

// buildMatch turns a user query into an FTS5 match expression.
//
// The words are quoted, so that a word with a character FTS5 treats as
// syntax cannot break the query. The last word gets a prefix wildcard:
// the user is usually still typing it.
//
// This is the forgiving way to search: every word of the query has to
// appear somewhere in the page. It is what a query the index cannot
// read falls back to.
func buildMatch(query string) string {
	words := queryWords(query)
	// A query that ends in an operator is not finished. The operator
	// is not a word the pages have to hold, so it is left out.
	if len(words) > 0 && operators[strings.ToUpper(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	if len(words) == 0 {
		return ""
	}

	parts := make([]string, 0, len(words))
	for idx, word := range words {
		quoted := `"` + strings.ReplaceAll(word, `"`, `""`) + `"`
		if idx == len(words)-1 {
			parts = append(parts, quoted+"*")
			continue
		}
		parts = append(parts, quoted)
	}
	return strings.Join(parts, " AND ")
}

// queryWords splits a query into the words to search for. Punctuation
// is dropped: FTS5 would treat it as syntax, and the user meant a word.
func queryWords(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	words := make([]string, 0, len(fields))
	for _, word := range fields {
		word = strings.ToLower(word)
		if word == "" {
			continue
		}
		words = append(words, word)
	}
	return words
}
