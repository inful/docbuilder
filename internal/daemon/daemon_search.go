package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/search"
)

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
func (d *Daemon) initSearch(cfg *config.Config) error {
	if cfg == nil || cfg.Search == nil || cfg.Search.Provider != config.SearchProviderServer {
		return nil
	}
	if cfg.Daemon == nil {
		// The search runs on the server, but there is no server.
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
func (s *SearchService) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.index == nil {
		http.Error(w, "search is not available", http.StatusServiceUnavailable)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	response := searchResponse{Query: query, Results: []search.Result{}}

	if query != "" {
		results, err := s.index.Search(r.Context(), query, s.maxResults)
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
