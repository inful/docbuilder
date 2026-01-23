package lint

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
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
	// Note: even when no git-based rename mappings are found, we may still be
	// able to heal certain broken links via conservative heuristics.

	// If the fixer already renamed files in this run (e.g., filename normalization),
	// make sure we heal links directly to the final on-disk destination.
	mappings = applyFixerRenameDestinations(mappings, fixResult)

	index := indexRenamesByOld(mappings)
	linksByMapping := collectLinksByMapping(f, brokenLinks, index, fixResult)
	if len(linksByMapping) == 0 {
		return
	}

	applyHealedLinkUpdates(f, linksByMapping, fixResult, fingerprintTargets)
}

func applyFixerRenameDestinations(mappings []RenameMapping, fixResult *FixResult) []RenameMapping {
	if fixResult == nil || len(fixResult.FilesRenamed) == 0 || len(mappings) == 0 {
		return mappings
	}

	byOld := make(map[string]string, len(fixResult.FilesRenamed))
	for _, op := range fixResult.FilesRenamed {
		if !op.Success {
			continue
		}
		byOld[strings.ToLower(normalizePathKey(op.OldPath))] = op.NewPath
	}
	if len(byOld) == 0 {
		return mappings
	}

	out := make([]RenameMapping, 0, len(mappings))
	for _, m := range mappings {
		m.NewAbs = resolveRenameChain(byOld, m.NewAbs)
		out = append(out, m)
	}
	return out
}

func resolveRenameChain(byOld map[string]string, startAbs string) string {
	cur := startAbs
	for range 10 {
		next, ok := byOld[strings.ToLower(normalizePathKey(cur))]
		if !ok {
			break
		}
		cur = next
	}
	return cur
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
	uncommittedDetector := &GitUncommittedRenameDetector{}
	uncommitted, err := uncommittedDetector.DetectRenames(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect git uncommitted renames: %w", err)
	}

	historyDetector := &GitHistoryRenameDetector{}
	history, err := historyDetector.DetectRenames(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect git history renames: %w", err)
	}

	combined := append(append([]RenameMapping(nil), uncommitted...), history...)
	if len(combined) == 0 {
		return nil, nil
	}

	normalized, err := NormalizeRenameMappings(combined, []string{docsRoot})
	if err != nil {
		return nil, fmt.Errorf("failed to normalize rename mappings: %w", err)
	}
	return normalized, nil
}

type renameIndex struct {
	exact  map[string][]RenameMapping
	folded map[string][]RenameMapping
}

func indexRenamesByOld(mappings []RenameMapping) renameIndex {
	idx := renameIndex{
		exact:  make(map[string][]RenameMapping, len(mappings)),
		folded: make(map[string][]RenameMapping, len(mappings)),
	}
	for _, m := range mappings {
		key := normalizePathKey(m.OldAbs)
		idx.exact[key] = append(idx.exact[key], m)
		idx.folded[strings.ToLower(key)] = append(idx.folded[strings.ToLower(key)], m)
	}
	return idx
}

func normalizePathKey(absPath string) string {
	return filepath.ToSlash(filepath.Clean(absPath))
}

func collectLinksByMapping(f *Fixer, brokenLinks []BrokenLink, idx renameIndex, fixResult *FixResult) map[mappingKey][]LinkReference {
	linksByMapping := make(map[mappingKey][]LinkReference)
	linkCache := make(map[string][]LinkReference)
	gitHeadDocCache := make(map[string][]docmodel.LinkRef)

	for _, bl := range brokenLinks {
		// Safety: broken link detection should have already filtered these, but
		// keep the healer defensive.
		if isHugoShortcodeLinkTarget(bl.Target) || isUIDAliasLinkTarget(bl.Target) {
			continue
		}
		if strings.HasPrefix(bl.Target, "http://") || strings.HasPrefix(bl.Target, "https://") || strings.HasPrefix(bl.Target, "mailto:") || strings.HasPrefix(bl.Target, "#") {
			continue
		}

		resolved, err := resolveRelativePath(bl.SourceFile, bl.Target)
		if err != nil {
			continue
		}

		mapping, ok, candidates := lookupUnambiguousRenameMapping(idx, resolved)
		if !ok {
			// Git-based rename detection can miss “round-trip” renames where a file is
			// renamed and later renamed back to its original path before commit. In
			// that case, there is no net rename vs HEAD/index, but the linking file
			// may still point at the transient name.
			//
			// Fallback (still conservative): look at the HEAD version of the linking
			// file and, if the corresponding link (same line + kind) pointed somewhere
			// that *now* resolves to an existing file, heal back to that destination.
			fallback, okFallback := inferRenameMappingFromGitHead(bl, resolved, gitHeadDocCache)
			if !okFallback {
				continue
			}
			mapping = fallback
		}
		if len(candidates) > 0 {
			fixResult.HealSkipped = append(fixResult.HealSkipped, BrokenLinkHealSkip{
				SourceFile: bl.SourceFile,
				LineNumber: bl.LineNumber,
				Target:     bl.Target,
				Reason:     "ambiguous git rename mapping",
				Candidates: candidates,
			})
			continue
		}
		if mapping.OldAbs == "" || mapping.NewAbs == "" {
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

func inferRenameMappingFromGitHead(bl BrokenLink, resolvedOldAbs string, headCache map[string][]docmodel.LinkRef) (RenameMapping, bool) {
	// We only handle markdown-style broken links here.
	if bl.SourceFile == "" || bl.LineNumber <= 0 {
		return RenameMapping{}, false
	}

	// Parse the current file so we can determine the link kind at this location.
	curDoc, err := docmodel.ParseFile(bl.SourceFile, docmodel.Options{})
	if err != nil {
		return RenameMapping{}, false
	}
	curRefs, err := curDoc.LinkRefs()
	if err != nil {
		return RenameMapping{}, false
	}

	curMatches := make([]docmodel.LinkRef, 0, 1)
	curPos := -1
	for i, ref := range curRefs {
		dest := strings.TrimSpace(ref.Link.Destination)
		if ref.FileLine != bl.LineNumber {
			continue
		}
		if dest != strings.TrimSpace(bl.Target) {
			continue
		}
		curMatches = append(curMatches, ref)
		curPos = i
	}
	if len(curMatches) != 1 || curPos < 0 {
		// Ambiguous or not found.
		return RenameMapping{}, false
	}
	curKind := curMatches[0].Link.Kind

	headRefs, ok := headCache[bl.SourceFile]
	if !ok {
		repoRoot := gitTopLevelOrSelf(context.Background(), filepath.Dir(bl.SourceFile))
		rel, relErr := filepath.Rel(repoRoot, bl.SourceFile)
		if relErr != nil {
			return RenameMapping{}, false
		}
		rel = filepath.ToSlash(rel)
		data, okShow := gitShowHeadFile(context.Background(), repoRoot, rel)
		if !okShow {
			return RenameMapping{}, false
		}

		headDoc, parseErr := docmodel.Parse(data, docmodel.Options{})
		if parseErr != nil {
			return RenameMapping{}, false
		}
		headRefs, err = headDoc.LinkRefs()
		if err != nil {
			return RenameMapping{}, false
		}
		headCache[bl.SourceFile] = headRefs
	}

	if curPos >= len(headRefs) {
		return RenameMapping{}, false
	}
	headRef := headRefs[curPos]
	if headRef.Link.Kind != curKind {
		return RenameMapping{}, false
	}
	headDest := strings.TrimSpace(headRef.Link.Destination)
	if headDest == "" {
		return RenameMapping{}, false
	}
	if isHugoShortcodeLinkTarget(headDest) || isUIDAliasLinkTarget(headDest) {
		return RenameMapping{}, false
	}
	if strings.HasPrefix(headDest, "http://") || strings.HasPrefix(headDest, "https://") || strings.HasPrefix(headDest, "mailto:") || strings.HasPrefix(headDest, "#") {
		return RenameMapping{}, false
	}
	if headDest == strings.TrimSpace(bl.Target) {
		return RenameMapping{}, false
	}
	// If the HEAD destination now resolves to an existing file, we can safely
	// heal back to it.
	newAbs, err := resolveRelativePath(bl.SourceFile, headDest)
	if err != nil {
		return RenameMapping{}, false
	}
	if !fileExists(newAbs) {
		return RenameMapping{}, false
	}

	return RenameMapping{OldAbs: resolvedOldAbs, NewAbs: newAbs, Source: RenameSourceGitHistory}, true
}

func gitShowHeadFile(ctx context.Context, repoRoot string, relPath string) ([]byte, bool) {
	if repoRoot == "" || relPath == "" {
		return nil, false
	}
	// #nosec G204 -- invoking git with fixed binary name and controlled args
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show", "HEAD:"+relPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	return out, true
}

// lookupUnambiguousRenameMapping returns a single mapping if it can identify a
// unique destination. If multiple distinct destinations match, candidates will
// be non-empty and the caller should skip applying a rewrite for safety.
func lookupUnambiguousRenameMapping(idx renameIndex, resolvedAbs string) (mapping RenameMapping, ok bool, candidates []string) {
	// Prefer exact matches to avoid false ambiguity when two files differ only
	// by case on case-sensitive filesystems.
	exact := lookupRenameMappings(idx.exact, resolvedAbs, false)
	if len(exact) > 0 {
		return selectUnambiguous(exact)
	}

	folded := lookupRenameMappings(idx.folded, resolvedAbs, true)
	if len(folded) > 0 {
		return selectUnambiguous(folded)
	}

	return RenameMapping{}, false, nil
}

func lookupRenameMappings(byOld map[string][]RenameMapping, resolvedAbs string, isFolded bool) []RenameMapping {
	var matches []RenameMapping
	seen := make(map[string]struct{})
	for _, c := range candidateOldPaths(resolvedAbs) {
		key := normalizePathKey(c)
		if isFolded {
			key = strings.ToLower(key)
		}
		for _, m := range byOld[key] {
			id := normalizePathKey(m.OldAbs) + "\x00" + normalizePathKey(m.NewAbs) + "\x00" + string(m.Source)
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			matches = append(matches, m)
		}
	}
	return matches
}

func selectUnambiguous(matches []RenameMapping) (RenameMapping, bool, []string) {
	if len(matches) == 0 {
		return RenameMapping{}, false, nil
	}

	uniqueNew := make(map[string]RenameMapping)
	for _, m := range matches {
		uniqueNew[normalizePathKey(m.NewAbs)] = m
	}

	if len(uniqueNew) == 1 {
		for _, m := range uniqueNew {
			return m, true, nil
		}
	}

	// Ambiguous: multiple candidate destinations.
	outs := make([]string, 0, len(uniqueNew))
	for newAbs := range uniqueNew {
		outs = append(outs, newAbs)
	}
	sort.Strings(outs)
	return RenameMapping{}, true, outs
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
