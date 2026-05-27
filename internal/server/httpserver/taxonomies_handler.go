package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"

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
	tags, categories, err := docs.CollectTaxonomiesFromContent(contentDir)
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
