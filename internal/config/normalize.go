package config

import (
    "fmt"
    "strings"
)

// NormalizationResult captures adjustments & warnings from normalization pass.
type NormalizationResult struct { Warnings []string }

// NormalizeConfig performs canonicalization on enumerated and bounded fields prior to default application.
// It mutates the provided config in-place and returns a result describing any coercions.
func NormalizeConfig(c *Config) (*NormalizationResult, error) {
    if c == nil { return nil, fmt.Errorf("config nil") }
    res := &NormalizationResult{}
    normalizeBuildConfig(&c.Build, res)
    return res, nil
}

func normalizeBuildConfig(b *BuildConfig, res *NormalizationResult) {
    if b == nil { return }
    // render_mode
    if rm := NormalizeRenderMode(string(b.RenderMode)); rm != "" {
        if b.RenderMode != rm { res.Warnings = append(res.Warnings, warnChanged("build.render_mode", b.RenderMode, rm)); b.RenderMode = rm }
    } else if strings.TrimSpace(string(b.RenderMode)) != "" {
        res.Warnings = append(res.Warnings, warnUnknown("build.render_mode", string(b.RenderMode), string(RenderModeAuto)))
        b.RenderMode = RenderModeAuto
    }
    // namespace_forges
    if nm := NormalizeNamespacingMode(string(b.NamespaceForges)); nm != "" {
        if b.NamespaceForges != nm { res.Warnings = append(res.Warnings, warnChanged("build.namespace_forges", b.NamespaceForges, nm)); b.NamespaceForges = nm }
    } else if strings.TrimSpace(string(b.NamespaceForges)) != "" {
        res.Warnings = append(res.Warnings, warnUnknown("build.namespace_forges", string(b.NamespaceForges), string(NamespacingAuto)))
        b.NamespaceForges = NamespacingAuto
    }
    // clone_strategy
    if cs := NormalizeCloneStrategy(string(b.CloneStrategy)); cs != "" {
        if b.CloneStrategy != cs { res.Warnings = append(res.Warnings, warnChanged("build.clone_strategy", b.CloneStrategy, cs)); b.CloneStrategy = cs }
    } else if strings.TrimSpace(string(b.CloneStrategy)) != "" {
        res.Warnings = append(res.Warnings, warnUnknown("build.clone_strategy", string(b.CloneStrategy), string(CloneStrategyFresh)))
        b.CloneStrategy = CloneStrategyFresh
    }
    // bounds
    if b.CloneConcurrency < 0 { b.CloneConcurrency = 0 }
    if b.ShallowDepth < 0 { b.ShallowDepth = 0 }
    // retry_backoff
    if rb := NormalizeRetryBackoff(string(b.RetryBackoff)); rb != "" {
        if b.RetryBackoff != rb { res.Warnings = append(res.Warnings, warnChanged("build.retry_backoff", b.RetryBackoff, rb)); b.RetryBackoff = rb }
    } else if strings.TrimSpace(string(b.RetryBackoff)) != "" {
        res.Warnings = append(res.Warnings, warnUnknown("build.retry_backoff", string(b.RetryBackoff), string(RetryBackoffFixed)))
        b.RetryBackoff = RetryBackoffFixed
    }
}

func warnChanged(field string, from, to interface{}) string { return fmt.Sprintf("normalized %s from '%v' to '%v'", field, from, to) }
func warnUnknown(field, value, def string) string { return fmt.Sprintf("unknown %s '%s', defaulting to %s", field, value, def) }