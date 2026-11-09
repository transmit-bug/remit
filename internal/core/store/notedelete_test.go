package store_test

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Renaming a note changes the name and nothing else: the file stays where it
// was minted, the blocks and the cards they feed are untouched, and the save is
// stamped with the moment the rename happened.
func TestRenameNoteKeepsItsFile(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Old name", "q :: a")
	path := notePath(dir, note)
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}

	at := clk.at.Add(2 * time.Hour)
	renamed, err := lib.RenameNote(note.ID, "New name", at)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Title != "New name" {
		t.Errorf("title = %q, want the new one", renamed.Title)
	}
	if !renamed.Updated.Equal(at) {
		t.Errorf("updated = %v, want the rename's moment %v", renamed.Updated, at)
	}
	if len(renamed.Blocks) != 1 || renamed.Blocks[0].ID != note.Blocks[0].ID {
		t.Errorf("the rename changed the blocks: %+v", renamed.Blocks)
	}
	// The slug was minted with the note, so the file did not move: a link or a
	// card that names the path still finds it.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the note's file moved: %v", err)
	}
	if _, ok := lib.Card(card.ID); !ok {
		t.Error("the card lost its source over a rename")
	}

	// A reopen reads the new title from the file, not from the index.
	next := reopen(t, lib, dir, clk)
	got, ok := next.Note(note.ID)
	if !ok || got.Title != "New name" {
		t.Fatalf("the reopened note is %q, %v", got.Title, ok)
	}
}

// Renaming a note that is not there is an error, not a new note: a caller that
// meant another id hears about it.
func TestRenameNoteNeedsTheNote(t *testing.T) {
	lib, _, _ := newLibrary(t)
	if _, err := lib.RenameNote(core.ID("note_missing"), "x", t0); err == nil {
		t.Error("renaming a note that is not there wrote one")
	}
}

// Deleting a note removes its file, its text and its place in every list, and
// leaves the cards it fed where they are: lost, with every review they were
// given still in the log. The lost cards are the re-link page's work, not a
// delete's.
func TestDeleteNoteLeavesItsCardsLost(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "To be deleted", "q :: a")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Review(card.ID, schedule.Good, clk.at); err != nil {
		t.Fatalf("review: %v", err)
	}

	if err := lib.DeleteNote(note.ID, clk.at, store.CascadeLose); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, ok := lib.Note(note.ID); ok {
		t.Error("the note is still readable")
	}
	if _, err := os.Stat(notePath(dir, note)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the note's file is still on disk: %v", err)
	}
	if _, ok := lib.Block(note.Blocks[0].ID); ok {
		t.Error("the note's block is still there")
	}
	if hits, _, err := lib.Search("To be deleted", "", 10, 0); err != nil || len(hits) != 0 {
		t.Errorf("the note is still searchable: %v, %d hits", err, len(hits))
	}

	// The card stays, loses its source, and keeps the review it was given.
	if _, ok := lib.Card(card.ID); !ok {
		t.Fatal("the card went with the note")
	}
	lost, err := lib.LostCards()
	if err != nil {
		t.Fatalf("lost: %v", err)
	}
	if len(lost) != 1 || lost[0].ID != card.ID {
		t.Errorf("lost cards = %v, want the deleted note's card %q", ids(lost), card.ID)
	}
	reviews, err := lib.LiveReviews(time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("reviews: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Entry() != card.ID {
		t.Errorf("the review log lost the card's history: %+v", reviews)
	}

	// A reopen agrees: the note is gone, the card is still lost, and its
	// schedule survived.
	next := reopen(t, lib, dir, clk)
	if _, ok := next.Note(note.ID); ok {
		t.Error("the reopened library still holds the note")
	}
	if state, ok := next.CardState(card.ID); !ok || state.Reviews != 1 {
		t.Errorf("the reopened card's state = %+v, %v", state, ok)
	}
	lost, err = next.LostCards()
	if err != nil {
		t.Fatalf("lost after reopen: %v", err)
	}
	if len(lost) != 1 || lost[0].ID != card.ID {
		t.Errorf("the reopened library's lost cards = %v, want %q", ids(lost), card.ID)
	}
}

// A note that is gone is out of the reading queue and out of its archive, in
// this library and in one opened from the same files.
func TestDeleteNoteLeavesTheReadingQueue(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	queued := saveNote(t, lib, "Queued", "read me")
	archived := saveNote(t, lib, "Archived", "read me too")
	for _, note := range []core.Note{queued, archived} {
		if err := lib.AddToReading(note.ID, clk.at); err != nil {
			t.Fatalf("queue %q: %v", note.Title, err)
		}
	}
	if err := lib.ArchiveReading(archived.ID, clk.at); err != nil {
		t.Fatalf("archive: %v", err)
	}

	for _, note := range []core.Note{queued, archived} {
		if err := lib.DeleteNote(note.ID, clk.at, store.CascadeLose); err != nil {
			t.Fatalf("delete %q: %v", note.Title, err)
		}
	}
	check := func(t *testing.T, lib *store.Library) {
		t.Helper()
		reading, err := lib.ReadingQueue()
		if err != nil {
			t.Fatalf("queue: %v", err)
		}
		if len(reading) != 0 {
			t.Errorf("the queue still lists %d deleted notes", len(reading))
		}
		archive, err := lib.ReadingArchive()
		if err != nil {
			t.Fatalf("archive: %v", err)
		}
		if len(archive) != 0 {
			t.Errorf("the archive still lists %d deleted notes", len(archive))
		}
	}
	check(t, lib)
	check(t, reopen(t, lib, dir, clk))
}

// Deleting a note is a decision the caller states: a cascade this build does
// not implement is refused before anything is written.
func TestDeleteNoteRefusesAnUnknownCascade(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Kept", "text")
	if err := lib.DeleteNote(note.ID, clk.at, store.Cascade(9)); err == nil {
		t.Fatal("an unknown cascade deleted the note")
	}
	if _, ok := lib.Note(note.ID); !ok {
		t.Error("the note went anyway")
	}
}
