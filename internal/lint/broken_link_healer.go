package lint

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type mappingKey struct {
	oldAbs string
	newAbs string
}

func (f *Fixer) healBrokenLinksFromGitRenames(rootPath string, brokenLinks []BrokenLink, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
	if f.dryRun {
		return
	}
	if len(brokenLinks) == 0 {
		return
	}

	docsRoot := docsRootFromPath(rootPath)
	repoDir := repoDirFromPath(rootPath)
	repoDir = gitTopLevelOrSelf(context.Background(), repoDir)

	mappings, err := detectScopedGitRenames(context.Background(), repoDir, docsRoot)
	if err != nil {
		fixResult.Errors = append(fixResult.Errors, err)
		return
	}
	if len(mappings) == 0 {
		return
	}

	byOld := indexRenamesByOld(mappings)
	linksByMapping := collectLinksByMapping(f, brokenLinks, byOld)
	if len(linksByMapping) == 0 {
		return
	}

	applyHealedLinkUpdates(f, linksByMapping, fixResult, fingerprintTargets)
}

func docsRootFromPath(path string) string {
	if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
		return filepath.Dir(path)
	}
	return path
}

func repoDirFromPath(path string) string {
	if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
		return filepath.Dir(path)
	}
	return path
}

func gitTopLevelOrSelf(ctx context.Context, dir string) string {
	if top, ok := gitTopLevelDir(ctx, dir); ok {
		return top
	}
	return dir
}

func detectScopedGitRenames(ctx context.Context, repoDir string, docsRoot string) ([]RenameMapping, error) {
	detector := &GitUncommittedRenameDetector{}
	mappings, err := detector.DetectRenames(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect git renames: %w", err)
	}
	if len(mappings) == 0 {
		return nil, nil
	}

	normalized, err := NormalizeRenameMappings(mappings, []string{docsRoot})
	if err != nil {
		return nil, fmt.Errorf("failed to normalize rename mappings: %w", err)
	}
	return normalized, nil
}

func indexRenamesByOld(mappings []RenameMapping) map[string]RenameMapping {
	byOld := make(map[string]RenameMapping, len(mappings))
	for _, m := range mappings {
		byOld[strings.ToLower(filepath.ToSlash(filepath.Clean(m.OldAbs)))] = m
	}
	return byOld
}

func collectLinksByMapping(f *Fixer, brokenLinks []BrokenLink, byOld map[string]RenameMapping) map[mappingKey][]LinkReference {
	linksByMapping := make(map[mappingKey][]LinkReference)
	linkCache := make(map[string][]LinkReference)

	for _, bl := range brokenLinks {
		resolved, err := resolveRelativePath(bl.SourceFile, bl.Target)
		if err != nil {
			continue
		}

		mapping, ok := lookupRenameMapping(byOld, resolved)
		if !ok {
			continue
		}

		cacheKey := bl.SourceFile + "\x00" + mapping.OldAbs
		references, ok := linkCache[cacheKey]
		if !ok {
			references, err = f.findLinksInFile(bl.SourceFile, mapping.OldAbs)
			if err != nil {
				continue
			}
			linkCache[cacheKey] = references
		}
		if len(references) == 0 {
			continue
		}

		mk := mappingKey{oldAbs: mapping.OldAbs, newAbs: mapping.NewAbs}
		linksByMapping[mk] = append(linksByMapping[mk], references...)
	}

	return linksByMapping
}

func lookupRenameMapping(byOld map[string]RenameMapping, resolvedAbs string) (RenameMapping, bool) {
	candidates := candidateOldPaths(resolvedAbs)
	for _, c := range candidates {
		key := strings.ToLower(filepath.ToSlash(filepath.Clean(c)))
		m, ok := byOld[key]
		if ok {
			return m, true
		}
	}
	return RenameMapping{}, false
}

func candidateOldPaths(resolvedAbs string) []string {
	candidates := []string{resolvedAbs}
	switch strings.ToLower(filepath.Ext(resolvedAbs)) {
	case "", ".html", ".htm":
		candidates = append(candidates, resolvedAbs+docExtensionMarkdown, resolvedAbs+docExtensionMarkdownLong)
	default:
		if hasKnownMarkdownExtension(resolvedAbs) {
			candidates = append(candidates, stripKnownMarkdownExtension(resolvedAbs))
		}
	}
	return candidates
}

func applyHealedLinkUpdates(f *Fixer, linksByMapping map[mappingKey][]LinkReference, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
	for mk, refs := range linksByMapping {
		updates, err := f.applyLinkUpdates(refs, mk.oldAbs, mk.newAbs)
		if err != nil {
			fixResult.Errors = append(fixResult.Errors, err)
			continue
		}

		fixResult.LinksUpdated = append(fixResult.LinksUpdated, updates...)
		pruneBrokenLinksFromUpdates(fixResult, updates)

		for _, upd := range updates {
			fingerprintTargets[upd.SourceFile] = struct{}{}
		}
	}
}

func pruneBrokenLinksFromUpdates(fixResult *FixResult, updates []LinkUpdate) {
	if len(updates) == 0 || len(fixResult.BrokenLinks) == 0 {
		return
	}

	fixed := make(map[string]struct{}, len(updates))
	for _, upd := range updates {
		fixed[upd.SourceFile+"\x00"+upd.OldTarget] = struct{}{}
	}

	remaining := make([]BrokenLink, 0, len(fixResult.BrokenLinks))
	for _, bl := range fixResult.BrokenLinks {
		if _, ok := fixed[bl.SourceFile+"\x00"+bl.Target]; ok {
			continue
		}
		remaining = append(remaining, bl)
	}
	fixResult.BrokenLinks = remaining
}

func gitTopLevelDir(ctx context.Context, dir string) (string, bool) {
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return "", false
	}
	return string(trimmed), true
}
