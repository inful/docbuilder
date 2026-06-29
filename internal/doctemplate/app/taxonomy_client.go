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

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
		return nil, nil, derrors.WrapError(err, derrors.CategoryValidation, "parse base URL").Build()
	}
	parsed.Path = path.Join(parsed.Path, "/api/taxonomies.json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryNetwork, "build taxonomy request").Build()
	}

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryNetwork, "fetch taxonomies").Build()
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, derrors.NewError(derrors.CategoryNetwork, fmt.Sprintf("fetch taxonomies: status %d", resp.StatusCode)).
			WithContext("status", resp.StatusCode).
			Build()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryNetwork, "read taxonomy response").Build()
	}

	var payload taxonomyResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryValidation, "decode taxonomy response").Build()
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
