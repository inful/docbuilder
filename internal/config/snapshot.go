package config

import (
	"crypto/sha256"
	"encoding/hex"
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
	// Build flags
	w("build.render_mode", string(c.Build.RenderMode))
	w("build.namespace_forges", string(c.Build.NamespaceForges))
	w("build.clone_strategy", string(c.Build.CloneStrategy))
	w("build.retry_backoff", string(c.Build.RetryBackoff))
	// Versioning
	if c.Versioning != nil {
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

func boolToString(b bool) string { return strconv.FormatBool(b) }
