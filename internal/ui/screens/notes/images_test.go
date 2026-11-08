package notes_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
)

const readImageSVG = `<svg viewBox="0 0 10 10"><rect width="10" height="10" fill="red"/></svg>`

// The read view keeps its Markdown options, so a picture is loaded once rather
// than once more on every frame.
func TestReadViewLoadsAnImageOnce(t *testing.T) {
	f := newFake()
	f.library = []notes.Note{{
		Note: core.Note{
			ID:    "note-image",
			Title: "Diagram",
			Blocks: []core.Block{{
				ID:   "block-image",
				Text: "![diagram](assets/diagram.svg)",
			}},
		},
	}}
	var reads atomic.Int32
	d := f.deps()
	d.GetAsset = func(rel string) ([]byte, bool) {
		if rel != "assets/diagram.svg" {
			return nil, false
		}
		reads.Add(1)
		return []byte(readImageSVG), true
	}

	s := notes.New()
	s.Selected = "note-image"
	s.View = notes.Read
	tt := ui.NewTester(func(c *ui.Context) { notes.View(c, &s, d) }, 1024, 720)
	tt.Frame()
	tt.Frame()

	deadline := time.Now().Add(2 * time.Second)
	for reads.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("the read view loaded the picture %d times across two frames, want once", got)
	}
	tt.Frame()
	time.Sleep(20 * time.Millisecond)
	if got := reads.Load(); got != 1 {
		t.Errorf("a later frame loaded the picture again: %d loads", got)
	}
}
