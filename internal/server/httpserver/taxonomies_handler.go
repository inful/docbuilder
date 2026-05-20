package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
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
	tags, categories, err := collectTaxonomiesFromContent(contentDir)
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

func collectTaxonomiesFromContent(contentDir string) ([]string, []string, error) {
	if st, err := os.Stat(contentDir); err != nil || !st.IsDir() {
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
		return []string{}, []string{}, nil
	}

	tagsSet := make(map[string]string)
	categoriesSet := make(map[string]string)

	err := filepath.WalkDir(contentDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		// #nosec G304,G122 -- path is discovered by walking a controlled local content directory.
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		fmRaw, _, had, _, splitErr := frontmatter.Split(content)
		if splitErr == nil && had {
			fields, parseErr := frontmatter.ParseYAML(fmRaw)
			if parseErr == nil {
				addTaxonomyValues(tagsSet, fields["tags"])
				addTaxonomyValues(categoriesSet, fields["categories"])
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	tags := sortedValues(tagsSet)
	categories := sortedValues(categoriesSet)
	return tags, categories, nil
}

func addTaxonomyValues(dst map[string]string, value any) {
	switch v := value.(type) {
	case string:
		addTaxonomyValue(dst, v)
	case []string:
		for _, item := range v {
			addTaxonomyValue(dst, item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				addTaxonomyValue(dst, s)
			}
		}
	}
}

func addTaxonomyValue(dst map[string]string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	key := strings.ToLower(trimmed)
	if _, exists := dst[key]; !exists {
		dst[key] = trimmed
	}
}

func sortedValues(src map[string]string) []string {
	if len(src) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, src[key])
	}
	return values
}
