package config

func normalizeFiltering(f *FilteringConfig, res *NormalizationResult) {
	if f == nil {
		return
	}
	f.RequiredPaths = normalizeStringSlice("filtering.required_paths", f.RequiredPaths, res)
	f.IgnoreFiles = normalizeStringSlice("filtering.ignore_files", f.IgnoreFiles, res)
	f.IncludePatterns = normalizeStringSlice("filtering.include_patterns", f.IncludePatterns, res)
	f.ExcludePatterns = normalizeStringSlice("filtering.exclude_patterns", f.ExcludePatterns, res)
}
