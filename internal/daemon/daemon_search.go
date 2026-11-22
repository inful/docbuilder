package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/search"
)

// defaultMaxResults is how many results a search answers with when
// the configuration says nothing.
const defaultMaxResults = 10

// SearchService keeps the full-text index of the rendered site and
// answers the queries of the docs server.
//
// The index lives in the daemon's state directory, so it survives a
// restart: the daemon does not have to wait for the next build to
// answer a query.
type SearchService struct {
	index      *search.Index
	maxResults int
}

// initSearch opens the search index when the configuration asks for the
// server-side search. Without it the site keeps the browser-side index
// the theme ships.
//
// The daemon serves the site it builds, so when nobody chose a provider
// the daemon chooses the server-side one. A plain build has no daemon
// behind it and keeps the browser-side index - see the config defaults.
func (d *Daemon) initSearch(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	if cfg.Daemon == nil {
		// The search runs on the server, but there is no server.
		return nil
	}
	if cfg.Search == nil {
		cfg.Search = &config.SearchConfig{}
	}
	if cfg.Search.Provider == "" {
		cfg.Search.Provider = config.SearchProviderServer
	}
	if cfg.Search.Provider != config.SearchProviderServer {
		return nil
	}

	stateDir := cfg.Daemon.Storage.StateDir
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}

	index, err := search.Open(filepath.Join(stateDir, "search"))
	if err != nil {
		return err
	}
	d.optional.search = &SearchService{index: index, maxResults: cfg.Search.MaxResults}
	return nil
}

// startIndexSearch writes the search index in the background.
//
// The index holds the pages of the last build: a page that is gone
// from the site is gone from the index with it. The walk over every
// page takes seconds on a big site, so it does not hold up the next
// build.
func (d *Daemon) startIndexSearch(ctx context.Context, outputDir string) {
	if d.optional.search == nil {
		return
	}
	d.lifecycle.workers.Go(func() {
		d.indexSearch(ctx, outputDir)
	})
}

// indexSearch re-indexes the pages of the last build.
//
// The index always holds the pages of the last build: a page that is
// gone from the site is gone from the index with it.
func (d *Daemon) indexSearch(ctx context.Context, outputDir string) {
	if d.optional.search == nil {
		return
	}

	start := time.Now()
	docs, err := search.Collect(outputDir)
	if err != nil {
		slog.Warn("Failed to collect the pages for the search index",
			logfields.Error(err), logfields.Path(outputDir))
		return
	}
	if err := d.optional.search.index.Upsert(ctx, docs); err != nil {
		slog.Warn("Failed to update the search index", logfields.Error(err))
		return
	}
	slog.Info("Search index updated",
		slog.Int("pages", len(docs)),
		slog.Duration("duration", time.Since(start)))
}

// searchResponse is the answer of the search endpoint.
type searchResponse struct {
	Query   string          `json:"query"`
	Results []search.Result `json:"results"`
}

// HandleSearch answers a search query.
//
// The query is answered by the daemon, so the visitor downloads the
// results and nothing else: the index of the whole site never reaches
// the browser.
//
// The visitor may ask for fewer results than the configuration allows,
// never for more: the limit of the configuration is the ceiling.
func (s *SearchService) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.index == nil {
		http.Error(w, "search is not available", http.StatusServiceUnavailable)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	response := searchResponse{Query: query, Results: []search.Result{}}

	if query != "" {
		results, err := s.index.Search(r.Context(), query, s.resultLimit(r))
		if err != nil {
			slog.Warn("Search failed", logfields.Error(err), slog.String("query", query))
			http.Error(w, "search failed", http.StatusInternalServerError)
			return
		}
		response.Results = results
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Warn("Failed to write the search response", logfields.Error(err))
	}
}

// resultLimit is how many results the answer holds.
//
// The visitor may ask for fewer results than the configuration allows,
// never for more: the limit of the configuration is the ceiling.
func (s *SearchService) resultLimit(r *http.Request) int {
	limit := s.maxResults
	if limit <= 0 {
		limit = defaultMaxResults
	}
	if asked, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && asked > 0 && asked < limit {
		limit = asked
	}
	return limit
}
