package hugo

import "git.home.luguber.info/inful/docbuilder/internal/docs"

// ContentTransformer represents a transformation stage on a markdown document's raw bytes.
// Future usage: a pipeline will apply a sequence of transformers (links, front matter, shortcodes, etc.).
type ContentTransformer interface {
    Apply(file docs.DocFile, content []byte) ([]byte, error)
}
