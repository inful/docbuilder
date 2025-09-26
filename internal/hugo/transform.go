package hugo

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"gopkg.in/yaml.v3"
)

// Page is the in-memory representation of a markdown document being transformed.
type Page struct {
	File           docs.DocFile
	Raw            []byte         // Original raw bytes (unchanged)
	Content        string         // Mutable content body (without front matter once parsed)
	FrontMatter    map[string]any // Parsed or synthesized front matter (mutable)
	HadFrontMatter bool           // Whether original file had front matter
}

// ContentTransformer applies a transformation to a Page.
type ContentTransformer interface {
	Name() string
	Transform(p *Page) error
}

// TransformerPipeline runs a sequence of transformers.
type TransformerPipeline struct{ stages []ContentTransformer }

func NewTransformerPipeline(stages ...ContentTransformer) *TransformerPipeline {
	return &TransformerPipeline{stages: stages}
}

func (tp *TransformerPipeline) Run(p *Page) error {
	for _, st := range tp.stages {
		if err := st.Transform(p); err != nil {
			return fmt.Errorf("transform %s failed: %w", st.Name(), err)
		}
	}
	return nil
}

// RelativeLinkRewriter transformer
type RelativeLinkRewriter struct{}

func (r *RelativeLinkRewriter) Name() string { return "relative_link_rewriter" }
func (r *RelativeLinkRewriter) Transform(p *Page) error {
	p.Content = RewriteRelativeMarkdownLinks(p.Content)
	return nil
}

// FrontMatterParser extracts existing front matter (if any) and strips it from content.
type FrontMatterParser struct{}

func (f *FrontMatterParser) Name() string { return "front_matter_parser" }
func (f *FrontMatterParser) Transform(p *Page) error {
	body := p.Content
	if strings.HasPrefix(body, "---\n") {
		// Locate end delimiter. Accept both \n---\n and trailing ---\nEOF (last block)
		search := body[4:]
		if idx := strings.Index(search, "\n---\n"); idx >= 0 { // standard case
			fmContent := search[:idx]
			fm := map[string]any{}
			if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
				slog.Warn("Failed to parse existing front matter", "file", p.File.RelativePath, "error", err)
			} else {
				p.FrontMatter = fm
				p.HadFrontMatter = true
			}
			p.Content = search[idx+5:]
		}
	}
	return nil
}

// FrontMatterBuilder populates or augments front matter using existing map.
type FrontMatterBuilder struct{ ConfigProvider func() *Generator }

func (f *FrontMatterBuilder) Name() string { return "front_matter_builder" }
func (f *FrontMatterBuilder) Transform(p *Page) error {
	gen := f.ConfigProvider()
	built := BuildFrontMatter(FrontMatterInput{File: p.File, Existing: p.FrontMatter, Config: gen.config, Now: time.Now()})
	p.FrontMatter = built
	return nil
}

// FinalFrontMatterSerializer serializes front matter + content back to bytes in Page.Raw for writing.
type FinalFrontMatterSerializer struct{}

func (s *FinalFrontMatterSerializer) Name() string { return "front_matter_serialize" }
func (s *FinalFrontMatterSerializer) Transform(p *Page) error {
	fmData, err := yaml.Marshal(p.FrontMatter)
	if err != nil {
		return err
	}
	combined := fmt.Sprintf("---\n%s---\n%s", string(fmData), p.Content)
	p.Raw = []byte(combined)
	return nil
}

// EditLinkInjector ensures an editURL exists (when theme expects it) after front matter build but before serialization.
// It mirrors the logic in BuildFrontMatter but only runs if editURL is still absent; this allows it to be inserted
// flexibly in future pipelines without duplicating site-level param logic.
type EditLinkInjector struct{ ConfigProvider func() *Generator }

func (e *EditLinkInjector) Name() string { return "edit_link_injector" }
func (e *EditLinkInjector) Transform(p *Page) error {
	if p.FrontMatter == nil { // nothing to do
		return nil
	}
	if _, exists := p.FrontMatter["editURL"]; exists { // respect existing
		return nil
	}
	gen := e.ConfigProvider()
	if gen == nil || gen.config == nil || gen.config.Hugo.Theme != "hextra" { // currently only hextra per-page logic
		return nil
	}
	// Re-run minimal subset of logic from BuildFrontMatter (consider refactor to shared helper later)
	cfg := gen.config
	// Suppress if site base set
	if cfg.Hugo.Params != nil {
		if v, ok := cfg.Hugo.Params["editURL"]; ok {
			if m, ok := v.(map[string]any); ok {
				if b, ok := m["base"].(string); ok && b != "" {
					return nil
				}
			}
		}
	}
	var repoCfg *config.Repository
	for i := range cfg.Repositories {
		if cfg.Repositories[i].Name == p.File.Repository {
			repoCfg = &cfg.Repositories[i]
			break
		}
	}
	if repoCfg == nil { return nil }
	branch := repoCfg.Branch
	if branch == "" { branch = "main" }
	repoRel := p.File.RelativePath
	if base := strings.TrimSpace(p.File.DocsBase); base != "" && base != "." {
 		repoRel = filepath.ToSlash(filepath.Join(base, repoRel))
 	} else {
 		repoRel = filepath.ToSlash(repoRel)
 	}
 	url := strings.TrimSuffix(repoCfg.URL, ".git")
 	var editURL string
 	if strings.Contains(repoCfg.URL, "github.com") {
 		if strings.HasPrefix(url, "git@github.com:") { url = "https://github.com/" + strings.TrimPrefix(url, "git@github.com:") }
 		editURL = fmt.Sprintf("%s/edit/%s/%s", url, branch, repoRel)
 	} else if strings.Contains(repoCfg.URL, "gitlab.com") {
 		if strings.HasPrefix(url, "git@gitlab.com:") { url = "https://gitlab.com/" + strings.TrimPrefix(url, "git@gitlab.com:") }
 		editURL = fmt.Sprintf("%s/-/edit/%s/%s", url, branch, repoRel)
 	} else if strings.Contains(repoCfg.URL, "bitbucket.org") {
 		editURL = fmt.Sprintf("%s/src/%s/%s?mode=edit", url, branch, repoRel)
 	} else if strings.Contains(repoCfg.URL, "git.home.luguber.info") {
 		editURL = fmt.Sprintf("%s/_edit/%s/%s", url, branch, repoRel)
 	}
 	if editURL != "" { p.FrontMatter["editURL"] = editURL }
 	return nil
}
