package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

type taxonomyResponse struct {
	Tags       []string `json:"tags"`
	Categories []string `json:"categories"`
}

func fetchTaxonomies(ctx context.Context, baseURL string) ([]string, []string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		return nil, nil, errors.New("base URL is required")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return nil, nil, fmt.Errorf("parse base URL: %w", err)
	}
	parsed.Path = path.Join(parsed.Path, "/api/taxonomies.json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build taxonomy request: %w", err)
	}

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch taxonomies: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch taxonomies: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read taxonomy response: %w", err)
	}

	var payload taxonomyResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode taxonomy response: %w", err)
	}

	tags := normalizeTaxonomyValues(payload.Tags)
	categories := normalizeTaxonomyValues(payload.Categories)
	return tags, categories, nil
}

func normalizeTaxonomyValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}
