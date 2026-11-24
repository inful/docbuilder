package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Snapshot computes a stable hash of build-affecting normalized configuration fields.
// It is intentionally narrower than full serialization to avoid noisy rebuilds when
// unrelated config fields change. Slice fields are order-insensitive (sorted prior to hashing).
// Callers SHOULD run NormalizeConfig + applyDefaults before computing a snapshot to ensure
// canonical field values.
func (c *Config) Snapshot() string {
	if c == nil {
		return ""
	}
	h := sha256.New()
	w := func(parts ...string) { h.Write([]byte(strings.Join(parts, "="))); h.Write([]byte{0}) }
	// Hugo essentials
	w("hugo.base_url", c.Hugo.BaseURL)
	w("hugo.title", c.Hugo.Title)
	// The rest of the Hugo configuration decides what the site looks
	// like. A change of any of them has to rebuild the site: the daemon
	// serves the old site otherwise.
	w("hugo.description", c.Hugo.Description)
	w("hugo.enable_page_transitions", boolToString(c.Hugo.EnablePageTransitions))
	w("hugo.chrome", c.Hugo.Chrome)
	w("hugo.custom_css", c.Hugo.CustomCSS)
	w("hugo.params", sortedAnyMap(c.Hugo.Params))
	w("hugo.taxonomies", sortedMap(c.Hugo.Taxonomies))
	for _, name := range sortedKeys(c.Hugo.Menu) {
		items := c.Hugo.Menu[name]
		for i, item := range items {
			w("hugo.menu."+name, strconv.Itoa(i), item.Name, item.URL, strconv.Itoa(item.Weight))
		}
	}
	// The repositories decide which documents the site holds. The order
	// of the list does not matter: the site sorts the repositories by
	// name.
	repositories := append([]Repository{}, c.Repositories...)
	sort.Slice(repositories, func(i, j int) bool { return repositories[i].Name < repositories[j].Name })
	for i := range repositories {
		repository := &repositories[i]
		w("repository.name", repository.Name)
		w("repository.url", repository.URL)
		w("repository.branch", repository.Branch)
		w("repository.display_name", repository.DisplayName)
		w("repository.description", repository.Description)
		w("repository.paths", strings.Join(repository.Paths, ","))
		// The namespace and the tags of a repository decide the path of
		// every page of that repository: a change of them moves the
		// tree, and the site has to be built again.
		w("repository.namespace", repository.Namespace)
		for _, key := range sortedKeys(repository.Tags) {
			w("repository.tags."+key, repository.Tags[key])
		}
		if repository.Auth != nil {
			// The secret itself is not part of the hash: a secret that
			// changes does not change the site, and the repositories
			// that the secret gives access to are hashed on their own.
			w("repository.auth.type", string(repository.Auth.Type))
			w("repository.auth.username", repository.Auth.Username)
			w("repository.auth.key_path", repository.Auth.KeyPath)
		}
	}
	// Build flags
	w("build.render_mode", string(c.Build.RenderMode))
	w("build.namespace_forges", string(c.Build.NamespaceForges))
	w("build.clone_strategy", string(c.Build.CloneStrategy))
	w("build.retry_backoff", string(c.Build.RetryBackoff))
	// The limit of an asset decides whether the asset is copied at all.
	w("build.max_asset_size", strconv.FormatInt(c.Build.MaxAssetSize, 10))
	// Versioning
	if c.Versioning != nil {
		w("versioning.enabled", boolToString(c.Versioning.Enabled))
		w("versioning.default_branch_only", boolToString(c.Versioning.DefaultBranchOnly))
		w("versioning.strategy", string(c.Versioning.Strategy))
		w("versioning.max_versions", intToString(c.Versioning.MaxVersionsPerRepo))
		if len(c.Versioning.BranchPatterns) > 0 {
			bp := append([]string{}, c.Versioning.BranchPatterns...)
			sort.Strings(bp)
			w("versioning.branch_patterns", strings.Join(bp, ","))
		}
		if len(c.Versioning.TagPatterns) > 0 {
			tp := append([]string{}, c.Versioning.TagPatterns...)
			sort.Strings(tp)
			w("versioning.tag_patterns", strings.Join(tp, ","))
		}
	}
	// Output
	w("output.directory", c.Output.Directory)
	// Search provider: it decides what the site ships - the search
	// client and the index. A change must rebuild the site.
	if c.Search != nil {
		w("search.provider", c.Search.Provider)
	}
	// Daemon content policies (build-affecting when daemon config is present)
	if c.Daemon != nil {
		w("daemon.content.public_only", boolToString(c.Daemon.Content.PublicOnly))
	}
	// Filtering patterns (order-insensitive after normalization). These affect which repos/docs are included.
	if c.Filtering != nil {
		if len(c.Filtering.IncludePatterns) > 0 {
			ip := append([]string{}, c.Filtering.IncludePatterns...)
			sort.Strings(ip)
			w("filtering.include_patterns", strings.Join(ip, ","))
		}
		if len(c.Filtering.ExcludePatterns) > 0 {
			ep := append([]string{}, c.Filtering.ExcludePatterns...)
			sort.Strings(ep)
			w("filtering.exclude_patterns", strings.Join(ep, ","))
		}
		if len(c.Filtering.RequiredPaths) > 0 {
			rp := append([]string{}, c.Filtering.RequiredPaths...)
			sort.Strings(rp)
			w("filtering.required_paths", strings.Join(rp, ","))
		}
		if len(c.Filtering.IgnoreFiles) > 0 {
			ig := append([]string{}, c.Filtering.IgnoreFiles...)
			sort.Strings(ig)
			w("filtering.ignore_files", strings.Join(ig, ","))
		}
	}
	// Monitoring logging (affects runtime logging but not site content; included for completeness)
	if c.Monitoring != nil {
		w("monitoring.logging.level", string(c.Monitoring.Logging.Level))
		w("monitoring.logging.format", string(c.Monitoring.Logging.Format))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func intToString(i int) string { return strconv.Itoa(i) }

// sortedMap returns the entries of a map in a stable order.
func sortedMap(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, ",")
}

// sortedAnyMap returns the entries of a map of any values in a stable
// order.
func sortedAnyMap(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fmt.Sprintf("%v", values[key]))
	}
	return strings.Join(parts, ",")
}

// sortedKeys returns the keys of a map in a stable order.
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func boolToString(b bool) string { return strconv.FormatBool(b) }
