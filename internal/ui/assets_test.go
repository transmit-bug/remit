package ui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// pngBytes is one real PNG: a 2×1 red rectangle, decodable by the editor.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, img.RGBAAt(0, 0))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// notesEditor opens the notebook on a note and puts the caret in its body,
// the way a user reaches the editor.
func notesEditor(t *testing.T, a *App, tt *ui.Tester, title string) {
	t.Helper()
	notesMode(t, a, tt)
	openNote(t, tt, title)
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("focus the editor: %v", err)
	}
	tt.Advance(50 * time.Millisecond)
}

// noteBlocks is the note's blocks as the library holds them.
func noteBlocks(t *testing.T, a *App, id core.ID) []core.Block {
	t.Helper()
	note, ok := a.Library().Note(id)
	if !ok {
		t.Fatalf("note %s is gone", id)
	}
	return note.Blocks
}

func TestDroppedPictureIsStoredAndInserted(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesEditor(t, a, tt, seed.writing.Title)

	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(path, pngBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}

	if !tt.DropFiles(600, 400, path) {
		t.Fatalf("the editor did not take the drop")
	}
	tt.Frame()
	tt.Advance(320 * time.Millisecond)

	blocks := noteBlocks(t, a, seed.writing.ID)
	var refs []string
	for _, block := range blocks {
		if at := strings.Index(block.Text, "](assets/"); at >= 0 {
			end := strings.Index(block.Text[at:], ")")
			refs = append(refs, block.Text[at+2:at+end])
		}
	}
	if len(refs) != 1 {
		t.Fatalf("blocks %q carry %d picture references, want 1", blocks, len(refs))
	}

	// The reference names a file the library stored, byte for byte.
	rel := refs[0]
	data, ok, err := a.Library().Asset(rel)
	if err != nil || !ok {
		t.Fatalf("asset %q = (_, %v, %v)", rel, ok, err)
	}
	if !bytes.Equal(data, pngBytes(t)) {
		t.Errorf("stored asset is not the dropped bytes")
	}
	// The same picture dropped again stores nothing new.
	if !tt.DropFiles(600, 400, path) {
		t.Fatal("second drop refused")
	}
	tt.Frame()
	tt.Advance(320 * time.Millisecond)
	for _, block := range noteBlocks(t, a, seed.writing.ID) {
		if strings.Count(block.Text, "![]("+rel+")")+strings.Count(block.Text, "![](assets/") > 2 {
			t.Fatalf("a repeat drop wrote more than the two references: %q", block.Text)
		}
	}
}

func TestPastedPictureClipboardIsStoredAndInserted(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesEditor(t, a, tt, seed.writing.Title)

	// A screenshot's clipboard: a picture and nothing else, which the
	// editor's own paste would silently drop.
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, pngBytes(t)))))
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	tt.Advance(320 * time.Millisecond)

	blocks := noteBlocks(t, a, seed.writing.ID)
	pictures := 0
	for _, block := range blocks {
		pictures += strings.Count(block.Text, "](assets/")
	}
	if pictures != 1 {
		t.Fatalf("blocks %q hold %d picture references, want 1", blocks, pictures)
	}
}

func TestPasteWithoutAPictureKeepsTheTextPipeline(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesEditor(t, a, tt, seed.writing.Title)

	// Plain text on the clipboard: what the editor always pasted.
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("typed words")))))
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	tt.Advance(320 * time.Millisecond)
	tt.Frame()
	tt.Advance(320 * time.Millisecond)
	var text string
	for _, block := range noteBlocks(t, a, seed.writing.ID) {
		text += block.Text + "\n"
	}
	if !strings.Contains(text, "typed words") {
		t.Fatalf("blocks %q lost the pasted text", text)
	}
	if strings.Contains(text, "](assets/") {
		t.Fatalf("a text paste stored an asset: %q", text)
	}
}
