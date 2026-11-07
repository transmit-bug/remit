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
}
