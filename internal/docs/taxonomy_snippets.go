package docs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

const taxonomyBaseURLEnvVar = "DOCBUILDER_TEMPLATE_BASE_URL"

type vscodeSnippet struct {
	Prefix      any    `json:"prefix"`
	Body        string `json:"body"`
	Description string `json:"description"`
}

type taxonomiesResponse struct {
	Tags       []string `json:"tags"`
	Categories []string `json:"categories"`
}

// WriteVSCodeTaxonomySnippets writes VS Code markdown snippets for taxonomy
// values collected from generated content.
func WriteVSCodeTaxonomySnippets(ctx context.Context, contentDir, snippetsPath string) error {
	tags, categories, err := taxonomyValuesForSnippets(ctx, contentDir)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "collect taxonomies").
			WithContext("content_dir", contentDir).
			Build()
	}

	snippets := make(map[string]vscodeSnippet, len(tags)+len(categories))
	for _, category := range categories {
		snippets["Category: "+category] = vscodeSnippet{
			Prefix:      generateSnippetPrefixes(category),
			Body:        category,
			Description: "Category: " + category,
		}
	}
	for _, tag := range tags {
		snippets["Tag: "+tag] = vscodeSnippet{
			Prefix:      generateSnippetPrefixes(tag),
			Body:        tag,
			Description: "Tag: " + tag,
		}
	}

	if mkErr := os.MkdirAll(filepath.Dir(snippetsPath), 0o750); mkErr != nil {
		return derrors.WrapError(mkErr, derrors.CategoryFileSystem, "create snippets directory").
			WithContext("path", snippetsPath).
			Build()
	}

	data, err := json.MarshalIndent(snippets, "", "  ")
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "marshal snippets").
			WithContext("path", snippetsPath).
			Build()
	}
	data = append(data, '\n')

	if err := os.WriteFile(snippetsPath, data, 0o600); err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "write snippets file").
			WithContext("path", snippetsPath).
			Build()
	}

	return nil
}

func taxonomyValuesForSnippets(ctx context.Context, contentDir string) ([]string, []string, error) {
	if baseURL := strings.TrimSpace(os.Getenv(taxonomyBaseURLEnvVar)); baseURL != "" {
		return fetchTaxonomiesFromBaseURL(ctx, baseURL)
	}
	return CollectTaxonomiesFromContent(contentDir)
}

func fetchTaxonomiesFromBaseURL(ctx context.Context, baseURL string) ([]string, []string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryValidation, "parse base URL").Build()
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, nil, derrors.NewError(derrors.CategoryValidation, "unsupported base URL scheme").
			WithContext("scheme", parsed.Scheme).
			Build()
	}
	if parsed.Host == "" {
		return nil, nil, errors.New("base URL host is required")
	}
	parsed.Path = path.Join(parsed.Path, "/api/taxonomies.json")

	//nolint:gosec // URL comes from explicit user configuration via environment variable.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryNetwork, "build taxonomy request").Build()
	}

	client := &http.Client{Timeout: 4 * time.Second}
	//nolint:gosec // Target is restricted to validated http/https URL set by the local user.
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

	var payload taxonomiesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, derrors.WrapError(err, derrors.CategoryValidation, "decode taxonomy response").Build()
	}

	tags := normalizeSnippetTaxonomyValues(payload.Tags)
	categories := normalizeSnippetTaxonomyValues(payload.Categories)
	return tags, categories, nil
}

func normalizeSnippetTaxonomyValues(values []string) []string {
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

func generateSnippetPrefixes(name string) any {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return ""
	}

	prefixes := []string{lower}
	if strings.Contains(lower, " ") {
		concat := strings.ReplaceAll(lower, " ", "")
		if concat != lower {
			prefixes = append(prefixes, concat)
		}
	}

	if len(prefixes) == 1 {
		return prefixes[0]
	}

	return prefixes
}
