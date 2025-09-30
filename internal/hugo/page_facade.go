package hugo

// PageFacade represents the minimal contract content transforms require.
// It allows future decoupling of the concrete Page struct from transformer
// consumers and enables alternate page representations (e.g. streaming, lazy-loaded).
//
// Phase: 2 -> 3 extraction follow-up (scaffold only). Implementation will
// migrate current Page usage incrementally.
//
// NOTE: When adding methods here, update transforms registry adapter shims.
// Keep surface minimal—prefer explicit helper functions where possible.
type PageFacade interface {
	GetContent() string
	SetContent(string)
	GetOriginalFrontMatter() map[string]any
	SetOriginalFrontMatter(map[string]any, bool)
	AddPatch(p FrontMatterPatch)
	ApplyPatches()
	HadOriginalFrontMatter() bool
	Serialize() error
}

// Ensure *Page implements PageFacade (compile-time assertion)
var _ PageFacade = (*Page)(nil)

// Adapt *Page to PageFacade without exposing additional fields.
func (p *Page) GetContent() string                     { return p.Content }
func (p *Page) SetContent(s string)                    { p.Content = s }
func (p *Page) GetOriginalFrontMatter() map[string]any { return p.OriginalFrontMatter }
func (p *Page) SetOriginalFrontMatter(fm map[string]any, had bool) {
	p.OriginalFrontMatter = fm
	p.HadFrontMatter = had
}
func (p *Page) AddPatch(fp FrontMatterPatch) { p.Patches = append(p.Patches, fp) }
func (p *Page) ApplyPatches()                { p.applyPatches() }
func (p *Page) HadOriginalFrontMatter() bool { return p.HadFrontMatter }
// Serialize currently a no-op for the concrete Page in the legacy pipeline path because
// serialization is handled via the PageShim closure. Once the facade is the sole adapter,
// this method will encapsulate the YAML + body assembly.
func (p *Page) Serialize() error { return nil }
