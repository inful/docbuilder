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
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

	path := filepath.Join(dir, "search.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("search: open index: %w", err)
	}
	// The daemon serves search requests while a build writes to the
	// index, so readers must not block the writer and vice versa.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.ExecContext(context.Background(), pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("search: %s: %w", pragma, err)
		}
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
// Every word of the query has to appear somewhere in the page. The last
// word may be a prefix, so "deploy kube" finds "deployment/kubernetes"
// while the user is still typing. The words are stemmed the same way
// the index is, so "deploying" finds "deploy".
func (i *Index) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if i == nil || i.db == nil {
		return nil, errors.New("search: index is not open")
	}
	if limit <= 0 {
		limit = 10
	}

	match := buildMatch(query)
	if match == "" {
		return nil, nil
	}

	rows, err := i.db.QueryContext(ctx, searchStatement, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search: query %q: %w", query, err)
	}
	defer func() { _ = rows.Close() }()

	var results []Result
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

// buildMatch turns a user query into an FTS5 match expression.
//
// The words are quoted, so that a word with a character FTS5 treats as
// syntax cannot break the query. The last word gets a prefix wildcard:
// the user is usually still typing it.
func buildMatch(query string) string {
	words := queryWords(query)
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
