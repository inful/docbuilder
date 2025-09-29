package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// NormalizationResult captures adjustments & warnings from normalization pass.
type NormalizationResult struct{ Warnings []string }

// NormalizeConfig performs canonicalization on enumerated and bounded fields prior to default application.
// It mutates the provided config in-place and returns a result describing any coercions.
func NormalizeConfig(c *Config) (*NormalizationResult, error) {
	if c == nil {
		return nil, fmt.Errorf("config nil")
	}
	res := &NormalizationResult{}
	normalizeBuildConfig(&c.Build, res)
	normalizeMonitoring(&c.Monitoring, res)
	normalizeVersioning(c.Versioning, res)
	normalizeOutput(&c.Output, res)
	normalizeFiltering(c.Filtering, res)
	return res, nil
}

func normalizeBuildConfig(b *BuildConfig, res *NormalizationResult) {
	if b == nil {
		return
	}
	// render_mode
	if rm := NormalizeRenderMode(string(b.RenderMode)); rm != "" {
		if b.RenderMode != rm {
			res.Warnings = append(res.Warnings, warnChanged("build.render_mode", b.RenderMode, rm))
			b.RenderMode = rm
		}
	} else if strings.TrimSpace(string(b.RenderMode)) != "" {
		res.Warnings = append(res.Warnings, warnUnknown("build.render_mode", string(b.RenderMode), string(RenderModeAuto)))
		b.RenderMode = RenderModeAuto
	}
	// namespace_forges
	if nm := NormalizeNamespacingMode(string(b.NamespaceForges)); nm != "" {
		if b.NamespaceForges != nm {
			res.Warnings = append(res.Warnings, warnChanged("build.namespace_forges", b.NamespaceForges, nm))
			b.NamespaceForges = nm
		}
	} else if strings.TrimSpace(string(b.NamespaceForges)) != "" {
		res.Warnings = append(res.Warnings, warnUnknown("build.namespace_forges", string(b.NamespaceForges), string(NamespacingAuto)))
		b.NamespaceForges = NamespacingAuto
	}
	// clone_strategy
	if cs := NormalizeCloneStrategy(string(b.CloneStrategy)); cs != "" {
		if b.CloneStrategy != cs {
			res.Warnings = append(res.Warnings, warnChanged("build.clone_strategy", b.CloneStrategy, cs))
			b.CloneStrategy = cs
		}
	} else if strings.TrimSpace(string(b.CloneStrategy)) != "" {
		res.Warnings = append(res.Warnings, warnUnknown("build.clone_strategy", string(b.CloneStrategy), string(CloneStrategyFresh)))
		b.CloneStrategy = CloneStrategyFresh
	}
	// bounds
	if b.CloneConcurrency < 0 {
		b.CloneConcurrency = 0
	}
	if b.ShallowDepth < 0 {
		b.ShallowDepth = 0
	}
	// retry_backoff
	if rb := NormalizeRetryBackoff(string(b.RetryBackoff)); rb != "" {
		if b.RetryBackoff != rb {
			res.Warnings = append(res.Warnings, warnChanged("build.retry_backoff", b.RetryBackoff, rb))
			b.RetryBackoff = rb
		}
	} else if strings.TrimSpace(string(b.RetryBackoff)) != "" {
		res.Warnings = append(res.Warnings, warnUnknown("build.retry_backoff", string(b.RetryBackoff), string(RetryBackoffFixed)))
		b.RetryBackoff = RetryBackoffFixed
	}
}

func normalizeMonitoring(m **MonitoringConfig, res *NormalizationResult) {
	if m == nil || *m == nil {
		return
	}
	cfg := *m
	// Logging level
	if lvl := NormalizeLogLevel(string(cfg.Logging.Level)); lvl != "" {
		if cfg.Logging.Level != lvl {
			res.Warnings = append(res.Warnings, warnChanged("monitoring.logging.level", cfg.Logging.Level, lvl))
			cfg.Logging.Level = lvl
		}
	} else if string(cfg.Logging.Level) != "" {
		res.Warnings = append(res.Warnings, warnUnknown("monitoring.logging.level", string(cfg.Logging.Level), string(LogLevelInfo)))
		cfg.Logging.Level = LogLevelInfo
	}
	// Logging format
	if f := NormalizeLogFormat(string(cfg.Logging.Format)); f != "" {
		if cfg.Logging.Format != f {
			res.Warnings = append(res.Warnings, warnChanged("monitoring.logging.format", cfg.Logging.Format, f))
			cfg.Logging.Format = f
		}
	} else if string(cfg.Logging.Format) != "" {
		res.Warnings = append(res.Warnings, warnUnknown("monitoring.logging.format", string(cfg.Logging.Format), string(LogFormatText)))
		cfg.Logging.Format = LogFormatText
	}
}

func normalizeVersioning(v *VersioningConfig, res *NormalizationResult) {
	if v == nil {
		return
	}
	if st := NormalizeVersioningStrategy(string(v.Strategy)); st != "" {
		if v.Strategy != st {
			res.Warnings = append(res.Warnings, warnChanged("versioning.strategy", v.Strategy, st))
			v.Strategy = st
		}
	} else if string(v.Strategy) != "" { // user provided invalid string; leave for validator to catch
		// Do not coerce here—validator expects to reject; just record warning.
		res.Warnings = append(res.Warnings, fmt.Sprintf("invalid versioning.strategy '%s' (will fail validation)", v.Strategy))
	}
	// clamp max versions (0 or negative => unlimited: represent as 0)
	if v.MaxVersionsPerRepo < 0 {
		v.MaxVersionsPerRepo = 0
	}
	// trim patterns whitespace
	trimSlice := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, p := range in {
			if tp := strings.TrimSpace(p); tp != "" {
				out = append(out, tp)
			}
		}
		return out
	}
	v.BranchPatterns = trimSlice(v.BranchPatterns)
	v.TagPatterns = trimSlice(v.TagPatterns)
}

func normalizeOutput(o *OutputConfig, res *NormalizationResult) {
	if o == nil {
		return
	}
	// Clean path (remove trailing slashes, collapse ./) but keep relative vs absolute as provided.
	before := o.Directory
	if before == "" {
		return
	}
	cleaned := filepath.Clean(before)
	// filepath.Clean turns empty to "."; if user literally had "./site" we keep cleaned version.
	if cleaned != before {
		res.Warnings = append(res.Warnings, warnChanged("output.directory", before, cleaned))
		o.Directory = cleaned
	}
}

func normalizeFiltering(f *FilteringConfig, res *NormalizationResult) {
	if f == nil {
		return
	}
	// Helper: trim, drop empty, dedupe (case-sensitive), then stable sort.
	normSlice := func(label string, in []string) []string {
		if len(in) == 0 {
			return in
		}
		seen := make(map[string]struct{}, len(in))
		out := make([]string, 0, len(in))
		changed := false
		for _, v := range in {
			t := strings.TrimSpace(v)
			if t == "" {
				changed = true
				continue
			}
			if _, ok := seen[t]; ok {
				changed = true
				continue
			}
			if t != v {
				changed = true
			}
			seen[t] = struct{}{}
			out = append(out, t)
		}
		if changed {
			res.Warnings = append(res.Warnings, fmt.Sprintf("normalized filtering.%s list (%d -> %d entries)", label, len(in), len(out)))
		}
		if len(out) <= 1 {
			return out
		}
		// simple insertion sort (avoid extra import) for deterministic ordering
		for i := 1; i < len(out); i++ {
			j := i
			for j > 0 && out[j-1] > out[j] {
				out[j-1], out[j] = out[j], out[j-1]
				j--
			}
		}
		// If order changed relative to original (ignoring removed elements) we already flagged via changed.
		return out
	}
	f.RequiredPaths = normSlice("required_paths", f.RequiredPaths)
	f.IgnoreFiles = normSlice("ignore_files", f.IgnoreFiles)
	f.IncludePatterns = normSlice("include_patterns", f.IncludePatterns)
	f.ExcludePatterns = normSlice("exclude_patterns", f.ExcludePatterns)
}

func warnChanged(field string, from, to interface{}) string {
	return fmt.Sprintf("normalized %s from '%v' to '%v'", field, from, to)
}
func warnUnknown(field, value, def string) string {
	return fmt.Sprintf("unknown %s '%s', defaulting to %s", field, value, def)
}
