package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/core/transfer"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// A .remitlib opened from the file manager imports once and is opened again
// from its copy the second time.
func TestOpenArchiveImportsOnce(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	source := filepath.Join(t.TempDir(), "source")
	lib, err := store.Create(source, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if _, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Kind: core.KindBasic}}); err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	archive := filepath.Join(t.TempDir(), "Memory.remitlib")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if err := transfer.Write(source, f); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	f.Close()

	root := t.TempDir()
	app := appui.NewApp(appui.Options{LibraryDir: filepath.Join(root, "opened")})
	if err := openArchive(app, root, archive); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if got := len(app.Library().Cards()); got != 1 {
		t.Fatalf("imported library has %d cards, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(root, "Memory", "remit.json")); err != nil {
		t.Errorf("archive was not imported into root: %v", err)
	}

	if err := openArchive(app, root, archive); err != nil {
		t.Fatalf("second open: %v", err)
	}
	if got := len(app.Library().Cards()); got != 1 {
		t.Errorf("second open doubled the library: %d cards", got)
	}
}

func TestOpenArchiveRefusesOtherFiles(t *testing.T) {
	root := t.TempDir()
	app := appui.NewApp(appui.Options{LibraryDir: filepath.Join(root, "opened")})
	if err := openArchive(app, root, filepath.Join(root, "notes.md")); err == nil {
		t.Error("openArchive accepted a non-library file")
	}
}
