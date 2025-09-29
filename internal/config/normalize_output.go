package config

import "path/filepath"

func normalizeOutput(o *OutputConfig, res *NormalizationResult) {
	if o == nil {
		return
	}
	before := o.Directory
	if before == "" {
		return
	}
	cleaned := filepath.Clean(before)
	if cleaned != before {
		res.Warnings = append(res.Warnings, warnChanged("output.directory", before, cleaned))
		o.Directory = cleaned
	}
}
