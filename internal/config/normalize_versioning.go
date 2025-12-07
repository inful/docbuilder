package config

import "fmt"

func normalizeVersioning(v *VersioningConfig, res *NormalizationResult) {
	if v == nil {
		return
	}
	if st := NormalizeVersioningStrategy(string(v.Strategy)); st != "" {
		if v.Strategy != st {
			res.Warnings = append(res.Warnings, warnChanged("versioning.strategy", v.Strategy, st))
			v.Strategy = st
		}
	} else if string(v.Strategy) != "" { // invalid provided; leave for validation
		res.Warnings = append(res.Warnings, fmt.Sprintf("invalid versioning.strategy '%s' (will fail validation)", v.Strategy))
	}
	if v.MaxVersionsPerRepo < 0 {
		v.MaxVersionsPerRepo = 0
	}
	v.BranchPatterns = trimStringSlice(v.BranchPatterns)
	v.TagPatterns = trimStringSlice(v.TagPatterns)
}
