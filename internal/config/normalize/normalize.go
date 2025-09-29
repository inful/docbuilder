package normalize

import (
	"fmt"
	"strings"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// Result captures normalization adjustments and warnings (non-fatal).
type Result struct {
	Warnings []string
}

// Normalize applies canonicalization & defaulting transformations that logically occur
// after raw YAML load but before validation. It does NOT mutate fields required by validation
// semantics except to coerce user input (e.g., case-folding enumerations). Additional passes
// (versioning, monitoring) can be added via helper functions keeping this composable.
func Normalize(c *cfg.Config) (*Result, error) {
	if c == nil {
		return nil, fmt.Errorf("config nil")
	}
	res := &Result{}
	normalizeBuild(&c.Build, res)
	// Future: normalize monitoring, output, versioning.
	return res, nil
}

// normalizeBuild canonicalizes build-related enum fields & coerces numeric bounds.
func normalizeBuild(b *cfg.BuildConfig, res *Result) {
	if b == nil {
		return
	}
	// Render mode
	if rm := cfg.NormalizeRenderMode(string(b.RenderMode)); rm != "" {
		if b.RenderMode != rm {
			res.Warnings = append(res.Warnings, fmt.Sprintf("normalized build.render_mode to '%s'", rm))
			b.RenderMode = rm
		}
	} else if strings.TrimSpace(string(b.RenderMode)) != "" {
		res.Warnings = append(res.Warnings, fmt.Sprintf("unknown build.render_mode '%s', defaulting to auto", b.RenderMode))
		b.RenderMode = cfg.RenderModeAuto
	}
	// Namespacing
	if nm := cfg.NormalizeNamespacingMode(string(b.NamespaceForges)); nm != "" {
		if b.NamespaceForges != nm {
			res.Warnings = append(res.Warnings, fmt.Sprintf("normalized build.namespace_forges to '%s'", nm))
			b.NamespaceForges = nm
		}
	} else if strings.TrimSpace(string(b.NamespaceForges)) != "" {
		res.Warnings = append(res.Warnings, fmt.Sprintf("unknown build.namespace_forges '%s', defaulting to auto", b.NamespaceForges))
		b.NamespaceForges = cfg.NamespacingAuto
	}
	// Clone strategy (only canonicalize; default still applied in defaults pass)
	if cs := cfg.NormalizeCloneStrategy(string(b.CloneStrategy)); cs != "" {
		if b.CloneStrategy != cs {
			res.Warnings = append(res.Warnings, fmt.Sprintf("normalized build.clone_strategy to '%s'", cs))
			b.CloneStrategy = cs
		}
	} else if strings.TrimSpace(string(b.CloneStrategy)) != "" {
		res.Warnings = append(res.Warnings, fmt.Sprintf("unknown build.clone_strategy '%s', defaulting to fresh", b.CloneStrategy))
		b.CloneStrategy = cfg.CloneStrategyFresh
	}
	// Bounds
	if b.CloneConcurrency < 0 {
		b.CloneConcurrency = 0
	}
	if b.ShallowDepth < 0 {
		b.ShallowDepth = 0
	}
}
