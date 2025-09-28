package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// SkipStateAccess encapsulates the subset of state manager methods required to evaluate a skip.
type SkipStateAccess interface {
	GetRepoLastCommit(string) string
	GetLastConfigHash() string
	GetLastReportChecksum() string
	SetLastReportChecksum(string)
	GetRepoDocFilesHash(string) string
	GetLastGlobalDocFilesHash() string
	SetLastGlobalDocFilesHash(string)
}

// SkipEvaluator decides whether a build can be safely skipped based on
// persisted state + prior build report + filesystem probes.
type SkipEvaluator struct {
	outDir    string
	state     SkipStateAccess
	generator *hugo.Generator
}

// NewSkipEvaluator constructs a new evaluator.
func NewSkipEvaluator(outDir string, st SkipStateAccess, gen *hugo.Generator) *SkipEvaluator {
	return &SkipEvaluator{outDir: outDir, state: st, generator: gen}
}

// Evaluate returns (report, true) when the build can be skipped, otherwise (nil, false).
// It never returns an error; corrupt/missing data simply disables the skip and a full rebuild proceeds.
func (se *SkipEvaluator) Evaluate(repos []cfg.Repository) (*hugo.BuildReport, bool) {
	if se.state == nil || se.generator == nil || len(repos) == 0 {
		return nil, false
	}
	currentHash := se.generator.ComputeConfigHashForPersistence()
	if currentHash == "" || currentHash != se.state.GetLastConfigHash() {
		return nil, false
	}
	prevPath := filepath.Join(se.outDir, "build-report.json")
	data, err := os.ReadFile(prevPath)
	if err != nil { // no previous report => cannot skip
		return nil, false
	}
	// checksum verification
	sum := sha256.Sum256(data)
	prevSum := hex.EncodeToString(sum[:])
	if stored := se.state.GetLastReportChecksum(); stored != "" && stored != prevSum {
		slog.Warn("Previous build report checksum mismatch; forcing rebuild", "stored", stored, "current", prevSum)
		return nil, false
	}

	// public/ dir integrity
	publicDir := filepath.Join(se.outDir, "public")
	if fi, err := os.Stat(publicDir); err != nil || !fi.IsDir() {
		if err != nil {
			slog.Warn("Public directory missing; forcing rebuild", "dir", publicDir, "error", err)
		} else {
			slog.Warn("Public path not dir; forcing rebuild", "dir", publicDir)
		}
		return nil, false
	}
	if entries, err := os.ReadDir(publicDir); err != nil || len(entries) == 0 {
		if err != nil {
			slog.Warn("Failed to read public directory; forcing rebuild", "dir", publicDir, "error", err)
		} else {
			slog.Warn("Public directory empty; forcing rebuild", "dir", publicDir)
		}
		return nil, false
	}

	// Parse previous report
	prev := struct {
		Repositories  int    `json:"repositories"`
		Files         int    `json:"files"`
		RenderedPages int    `json:"rendered_pages"`
		DocFilesHash  string `json:"doc_files_hash"`
	}{}
	if err := json.Unmarshal(data, &prev); err != nil {
		slog.Warn("Failed to parse previous build report; forcing rebuild", "error", err)
		return nil, false
	}

	contentDir := filepath.Join(se.outDir, "content")
	// If there were files previously we run deeper probes.
	if prev.Files > 0 {
		contentStat, cErr := os.Stat(contentDir)
		// Validate global doc_files_hash consistency.
		if lastGlobal := se.state.GetLastGlobalDocFilesHash(); lastGlobal != "" && prev.DocFilesHash != "" && lastGlobal != prev.DocFilesHash {
			slog.Warn("Stored global doc_files_hash mismatch; forcing rebuild", "state", lastGlobal, "report", prev.DocFilesHash)
			return nil, false
		}
		if cErr != nil || !contentStat.IsDir() {
			if cErr != nil {
				slog.Warn("Content directory missing; forcing rebuild", "dir", contentDir, "error", cErr)
			} else {
				slog.Warn("Content path is not directory; forcing rebuild", "dir", contentDir)
			}
			return nil, false
		}
		// Probe for at least one markdown file.
		foundMD := false
		filepath.Walk(contentDir, func(p string, info os.FileInfo, err error) error {
			if err != nil || foundMD || info == nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				foundMD = true
			}
			return nil
		})
		if !foundMD {
			slog.Warn("No markdown files in content; forcing rebuild")
			return nil, false
		}

		// Per-repo hash presence / single-repo equivalence.
		missingPerRepo := false
		if len(repos) == 1 {
			rh := se.state.GetRepoDocFilesHash(repos[0].URL)
			if rh == "" || (prev.DocFilesHash != "" && rh != prev.DocFilesHash) {
				slog.Warn("Single repository doc_files_hash mismatch/missing; forcing rebuild", "repo", repos[0].URL, "repo_hash", rh, "report_hash", prev.DocFilesHash)
				missingPerRepo = true
			}
		} else {
			for _, r := range repos {
				if se.state.GetRepoDocFilesHash(r.URL) == "" {
					slog.Warn("Missing per-repo doc_files_hash; forcing rebuild", "repo", r.URL)
					missingPerRepo = true
					break
				}
			}
		}
		if missingPerRepo {
			return nil, false
		}
	} else { // prev.Files == 0
		// Only verify commit metadata below.
	}

	// All repos must have last commit metadata.
	for _, r := range repos {
		if se.state.GetRepoLastCommit(r.URL) == "" {
			slog.Warn("Missing last commit metadata; forcing rebuild", "repo", r.URL)
			return nil, false
		}
	}

	// Construct skip report reusing prior counts.
	report := &hugo.BuildReport{SchemaVersion: 1, Start: time.Now(), End: time.Now(), SkipReason: "no_changes", Outcome: hugo.OutcomeSuccess, Repositories: prev.Repositories, Files: prev.Files, RenderedPages: prev.RenderedPages, DocFilesHash: prev.DocFilesHash}
	if err := report.Persist(se.outDir); err == nil {
		if rb, rerr := os.ReadFile(prevPath); rerr == nil {
			hs := sha256.Sum256(rb)
			se.state.SetLastReportChecksum(hex.EncodeToString(hs[:]))
		}
		if report.DocFilesHash != "" {
			se.state.SetLastGlobalDocFilesHash(report.DocFilesHash)
		}
	} else {
		slog.Warn("Failed to persist skip report", "error", err)
	}
	slog.Info("Skipping build (unchanged) without cleaning output", "repos", report.Repositories, "files", report.Files, "content_probe", "ok")
	return report, true
}
