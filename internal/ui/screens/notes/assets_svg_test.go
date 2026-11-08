package notes

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
)

const diagramSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
  <rect width="10" height="10" fill="red"/>
</svg>`

// An SVG asset reaches the editor as an image, whether the note was reopened
// or the file was just inserted at the caret.
func TestSVGAssetReachesTheEditor(t *testing.T) {
	deps := Deps{GetAsset: func(rel string) ([]byte, bool) {
		if rel != "assets/diagram.svg" {
			return nil, false
		}
		return []byte(diagramSVG), true
	}}

	t.Run("reopened note", func(t *testing.T) {
		s := New()
		s.Draft = core.Note{
			ID:    "note",
			Title: "Diagram",
			Blocks: []core.Block{{
				ID:   "block",
				Text: "![diagram](assets/diagram.svg)",
			}},
		}
		s.deps = &deps
		s.ensureDocument()

		assertSVGImage(t, s.Doc.Text())
	})

	t.Run("inserted asset", func(t *testing.T) {
		s := New()
		s.Doc = richtext.NewDocument(richtext.Text{})
		s.insertAsset(nil, deps, "assets/diagram.svg", "diagram")

		assertSVGImage(t, s.Doc.Text())
	})
}

func assertSVGImage(t *testing.T, text richtext.Text) {
	t.Helper()
	for _, p := range text.Paragraphs() {
		for _, r := range p.Runs {
			if at := r.Attributes.Attachment; at != nil {
				if _, ok := at.Image.(*ui.SVG); !ok {
					t.Fatalf("the SVG asset became %T, want *ui.SVG", at.Image)
				}
				return
			}
		}
	}
	t.Fatal("the SVG asset stayed a name box")
}
