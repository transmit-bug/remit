package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// The two commands move a library in and out without a window, and the copy
// that comes back keeps its schedule.
func TestExportAndImport(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	source := filepath.Join(t.TempDir(), "library")
	lib, err := store.Create(source, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "Memory", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	cards, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Review(cards[0].ID, schedule.Good, now); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	archive := filepath.Join(t.TempDir(), "library.remitlib")
	if !runOffline(source, archive, "") {
		t.Fatal("export did nothing")
	}

	target := filepath.Join(t.TempDir(), "copy")
	if !runOffline(target, "", archive) {
		t.Fatal("import did nothing")
	}

	restored, err := store.Open(target, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	defer restored.Close()
	if got := len(restored.Cards()); got != 1 {
		t.Fatalf("copy has %d cards, want 1", got)
	}
	state, ok := restored.CardState(restored.Cards()[0].ID)
	if !ok || state.Reviews != 1 {
		t.Errorf("copy lost the review history: %+v (ok %v)", state, ok)
	}
}

func TestNoFlagsDoesNothing(t *testing.T) {
	if runOffline(t.TempDir(), "", "") {
		t.Error("runOffline acted with no command given")
	}
	if runMarkdown(t.TempDir(), "", "") {
		t.Error("runMarkdown acted with no command given")
	}
}

// The Markdown commands move notes in and out without a window: an export's
// files import back as the same notes, markers intact.
func TestMarkdownExportAndImport(t *testing.T) {
	source := filepath.Join(t.TempDir(), "library")
	lib, err := store.Create(source, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "Memory", Blocks: []core.Block{
		{Text: "spacing :: beats cramming"},
		{Text: "a second block"},
	}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	lib.Close()

	out := filepath.Join(t.TempDir(), "md")
	if !runMarkdown(source, "", out) {
		t.Fatal("export did nothing")
	}
	entries, err := filepath.Glob(filepath.Join(out, "*.md"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("export wrote %d files (%v), want 1", len(entries), err)
	}

	// Into a fresh library: the marked file brings its identity along.
	target := filepath.Join(t.TempDir(), "copy")
	if !runMarkdown(target, out, "") {
		t.Fatal("import did nothing")
	}
	restored, err := store.Open(target, store.Options{})
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	defer restored.Close()
	notes := restored.Notes()
	if len(notes) != 1 || notes[0].Title != "Memory" {
		t.Fatalf("copy holds %+v, want the one note", notes)
	}
	// Identity travels as block markers: cards and backlinks point at blocks,
	// not at the note's file name.
	if len(notes[0].Blocks) != 2 || notes[0].Blocks[0].ID != note.Blocks[0].ID ||
		notes[0].Blocks[0].Text != "spacing :: beats cramming" {
		t.Errorf("copy's blocks = %+v, want the file's markers and text", notes[0].Blocks)
	}

	// Again: the same files add nothing.
	if !runMarkdown(target, out, "") {
		t.Fatal("second import did nothing")
	}
	if got := len(restored.Notes()); got != 1 {
		t.Errorf("second import left %d notes, want 1", got)
	}
}
