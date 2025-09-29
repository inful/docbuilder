package config

import (
    "fmt"
    "strings"
)

func normalizeVersioning(v *VersioningConfig, res *NormalizationResult) {
    if v == nil { return }
    if st := NormalizeVersioningStrategy(string(v.Strategy)); st != "" {
        if v.Strategy != st {
            res.Warnings = append(res.Warnings, warnChanged("versioning.strategy", v.Strategy, st))
            v.Strategy = st
        }
    } else if string(v.Strategy) != "" { // invalid provided; leave for validation
        res.Warnings = append(res.Warnings, fmt.Sprintf("invalid versioning.strategy '%s' (will fail validation)", v.Strategy))
    }
    if v.MaxVersionsPerRepo < 0 { v.MaxVersionsPerRepo = 0 }
    trimSlice := func(in []string) []string {
        out := make([]string, 0, len(in))
        for _, p := range in {
            if tp := strings.TrimSpace(p); tp != "" { out = append(out, tp) }
        }
        return out
    }
    v.BranchPatterns = trimSlice(v.BranchPatterns)
    v.TagPatterns = trimSlice(v.TagPatterns)
}
