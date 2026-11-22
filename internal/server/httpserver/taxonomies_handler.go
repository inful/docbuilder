package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

type taxonomiesResponse struct {
	Tags       []string `json:"tags"`
	Categories []string `json:"categories"`
}

func (s *Server) handleTaxonomiesJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := taxonomiesResponse{
		Tags:       []string{},
		Categories: []string{},
	}

	contentDir := filepath.Join(s.resolveAbsoluteOutputDir(), "content")
	tags, categories, err := s.siteTaxonomies(contentDir)
	if err == nil {
		resp.Tags = tags
		resp.Categories = categories
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("Failed to encode taxonomies JSON", "error", err)
	}
}

// siteTaxonomies returns the tags and the categories of the site.
//
// They are read from the files of the site, and the answer is kept
// until the content directory changes: the build replaces the whole
// directory, so the time of the directory changes with it. Without the
// answer, every request read and parsed every file of the site.
func (s *Server) siteTaxonomies(contentDir string) ([]string, []string, error) {
	key := ""
	if info, err := os.Stat(contentDir); err == nil {
		key = info.ModTime().UTC().Format(time.RFC3339Nano)
	}

	s.taxonomiesMu.Lock()
	defer s.taxonomiesMu.Unlock()

	if key != "" && key == s.taxonomiesKey {
		return s.taxonomiesValue, s.taxonomiesCats, nil
	}

	tags, categories, err := docs.CollectTaxonomiesFromContent(contentDir)
	if err != nil {
		return nil, nil, err
	}
	s.taxonomiesKey = key
	s.taxonomiesValue = tags
	s.taxonomiesCats = categories
	return tags, categories, nil
}
