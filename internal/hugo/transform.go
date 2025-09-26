package hugo

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

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
