package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// taggedLibrary writes two notes carrying tags, one of them twice.
func taggedLibrary(t *testing.T) (*store.Library, string, core.Note, core.Note) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	spacing, err := lib.SaveNote(core.Note{Title: "Spacing", Blocks: []core.Block{
		{Text: "the gaps between reviews #learning"},
		{Text: "and again #learning #spacing"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	habit, err := lib.SaveNote(core.Note{Title: "Habit", Blocks: []core.Block{
		{Text: "one short sitting #habit"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return lib, dir, spacing, habit
}

// A tag is indexed with the note that carries it, once per note however often it
// appears.
func TestTagsFollowTheSave(t *testing.T) {
	lib, _, spacing, _ := taggedLibrary(t)
	defer lib.Close()

	counts := lib.Tags()
	if len(counts) != 3 {
		t.Fatalf("the library has %d tags, want 3: %+v", len(counts), counts)
	}
	for _, count := range counts {
		if count.Tag == "learning" && count.Notes != 1 {
			t.Errorf("learning is on %d notes, want 1", count.Notes)
		}
	}

	notes := lib.NotesWithTag("learning")
	if len(notes) != 1 || notes[0] != spacing.ID {
		t.Errorf("learning is on %v, want the spacing note", notes)
	}

	// Taking the tag out takes it out of the index.
	spacing.Blocks[0].Text = "the gaps between reviews"
	spacing.Blocks[1].Text = "and again #spacing"
	if _, err := lib.SaveNote(spacing); err != nil {
		t.Fatal(err)
	}
	if notes := lib.NotesWithTag("learning"); len(notes) != 0 {
		t.Errorf("the tag outlived its text: %v", notes)
	}
}

// A tag counts the notes that carry it and the cards their blocks feed. The
// counts are distinct on both sides: a tag repeated across a note's blocks is
// one note, and two cards on one tagged block are two cards.
func TestTagsCountNotesAndCards(t *testing.T) {
	lib, _, spacing, habit := taggedLibrary(t)
	defer lib.Close()

	if _, err := lib.CreateCards(spacing.Blocks[0].ID, []core.Derived{
		{Type: core.TypeBasic, Ordinal: 0},
		{Type: core.TypeBidirectional, Ordinal: 1},
	}); err != nil {
		t.Fatalf("cards for the tagged block: %v", err)
	}
	if _, err := lib.CreateCards(habit.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}}); err != nil {
		t.Fatalf("card for the habit note: %v", err)
	}

	want := map[string]store.TagCount{
		"learning": {Tag: "learning", Notes: 1, Cards: 2},
		"spacing":  {Tag: "spacing", Notes: 1, Cards: 0},
		"habit":    {Tag: "habit", Notes: 1, Cards: 1},
	}
	counts := lib.Tags()
	if len(counts) != len(want) {
		t.Fatalf("the library has %d tags, want %d: %+v", len(counts), len(want), counts)
	}
	for _, count := range counts {
		if count != want[count.Tag] {
			t.Errorf("%s is %+v, want %+v", count.Tag, count, want[count.Tag])
		}
	}
}

// The index is a cache: deleting it and reopening rebuilds the same tags.
func TestTagsSurviveARebuiltIndex(t *testing.T) {
	lib, dir, _, _ := taggedLibrary(t)
	before := lib.Tags()
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".remit", "index.sqlite")); err != nil {
		t.Fatalf("delete the index: %v", err)
	}
	again, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()

	after := again.Tags()
	if len(after) != len(before) {
		t.Fatalf("the rebuilt index holds %d tags, was %d", len(after), len(before))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("tag %d is %+v, was %+v", i, after[i], before[i])
		}
	}
}
