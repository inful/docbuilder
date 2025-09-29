package config

import "fmt"

import "strings"

func normalizeFiltering(f *FilteringConfig, res *NormalizationResult) {
    if f == nil { return }
    normSlice := func(label string, in []string) []string {
        if len(in) == 0 { return in }
        seen := make(map[string]struct{}, len(in))
        out := make([]string, 0, len(in))
        changed := false
        for _, v := range in {
            t := strings.TrimSpace(v)
            if t == "" { changed = true; continue }
            if _, ok := seen[t]; ok { changed = true; continue }
            if t != v { changed = true }
            seen[t] = struct{}{}
            out = append(out, t)
        }
        if changed { res.Warnings = append(res.Warnings, fmt.Sprintf("normalized filtering.%s list (%d -> %d entries)", label, len(in), len(out))) }
        if len(out) <= 1 { return out }
        for i := 1; i < len(out); i++ { j := i; for j > 0 && out[j-1] > out[j] { out[j-1], out[j] = out[j], out[j-1]; j-- } }
        return out
    }
    f.RequiredPaths = normSlice("required_paths", f.RequiredPaths)
    f.IgnoreFiles = normSlice("ignore_files", f.IgnoreFiles)
    f.IncludePatterns = normSlice("include_patterns", f.IncludePatterns)
    f.ExcludePatterns = normSlice("exclude_patterns", f.ExcludePatterns)
}
