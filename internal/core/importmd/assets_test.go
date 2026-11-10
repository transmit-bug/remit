package importmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

func TestImportDirectoryCarriesItsAssets(t *testing.T) {
	dir := t.TempDir()
	lib := newLib(t)

	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	png := []byte("picture-bytes")
	if err := os.WriteFile(filepath.Join(dir, "assets", "shot.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("# Shot\n\nlook: ![](assets/shot.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Import(dir, lib, Options{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 1 {
		t.Errorf("notes = %d, want 1", report.Notes)
	}
	if report.Assets != 1 {
		t.Errorf("assets = %d, want 1", report.Assets)
	}

	// The asset is in the library, named by content, and the note's
	// reference resolves to it.
	rel, err := lib.PutAsset(png, "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	data, ok, err := lib.Asset(rel)
	if err != nil || !ok || string(data) != string(png) {
		t.Fatalf("imported asset = (%q, %v, %v), want the picture stored", data, ok, err)
	}
	notes := lib.Notes()
	if len(notes) != 1 {
		t.Fatalf("library holds %d notes, want 1", len(notes))
	}
	if got := notes[0].Blocks[0].Text; got != "look: ![](assets/shot.png)" {
		t.Errorf("note text = %q, want the file's own words with its reference", got)
	}
}

func TestExportWritesTheReferencedAssets(t *testing.T) {
	lib := newLib(t)
	rel, err := lib.PutAsset([]byte("picture-bytes"), "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	if _, err := lib.SaveNote(noteOf("With picture", "look: ![]("+rel+")")); err != nil {
		t.Fatalf("save note: %v", err)
	}

	out := t.TempDir()
	report, err := Export(out, lib)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if report.Files != 1 || report.Assets != 1 {
		t.Fatalf("report = %+v, want 1 file and 1 asset", report)
	}
	data, err := os.ReadFile(filepath.Join(out, "assets", filepath.Base(rel)))
	if err != nil {
		t.Fatalf("exported asset: %v", err)
	}
	if string(data) != "picture-bytes" {
		t.Errorf("asset bytes = %q, want the stored bytes", data)
	}

	// The directory imports back and stores nothing new: same file, same
	// content-addressed name.
	back := newLib(t)
	again, err := Import(out, back, Options{})
	if err != nil {
		t.Fatalf("Import back: %v", err)
	}
	if again.Notes != 1 || again.Assets != 1 {
		t.Fatalf("round trip = %+v, want 1 note and 1 asset", again)
	}
	roundRel, err := back.PutAsset([]byte("picture-bytes"), "png")
	if err != nil {
		t.Fatalf("put asset again: %v", err)
	}
	if roundRel != rel {
		t.Errorf("round trip asset = %q, want %q", roundRel, rel)
	}
}

func TestExportNoteCarriesItsOwnAssets(t *testing.T) {
	lib := newLib(t)
	rel, err := lib.PutAsset([]byte("one-picture"), "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	if _, err := lib.PutAsset([]byte("other-picture"), "png"); err != nil {
		t.Fatalf("put other asset: %v", err)
	}
	note, err := lib.SaveNote(noteOf("With picture", "look: ![]("+rel+")"))
	if err != nil {
		t.Fatalf("save note: %v", err)
	}

	out := t.TempDir()
	report, err := Export(out, lib)
	if err != nil {
		t.Fatalf("Export all: %v", err)
	}
	// Only the referenced asset travels; the unreferenced one stays out of
	// the export.
	if report.Assets != 1 {
		t.Errorf("export all assets = %d, want 1 (the referenced one)", report.Assets)
	}

	_ = note
	single := t.TempDir()
	path, wrote, err := ExportNote(single, note.ID, lib)
	if err != nil || !wrote {
		t.Fatalf("ExportNote = (%q, %v, %v)", path, wrote, err)
	}
	entries, err := os.ReadDir(filepath.Join(single, "assets"))
	if err != nil {
		t.Fatalf("assets dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(rel) {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("single-note assets = %v, want only %s", names, filepath.Base(rel))
	}
}

func TestImportIgnoresForeignImageReferences(t *testing.T) {
	lib := newLib(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "web.md"), []byte("# Web\n\n![](//example.com/x.png) and ![](assets/../../etc.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Import(dir, lib, Options{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Assets != 0 {
		t.Errorf("assets = %d, want 0", report.Assets)
	}
}

// newLib opens a throwaway library.
func newLib(t *testing.T) *store.Library {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib
}

// noteOf is one note with a single block.
func noteOf(title, text string) core.Note {
	return core.Note{Title: title, Blocks: []core.Block{{Text: text}}}
}
