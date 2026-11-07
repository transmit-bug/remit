package store_test

import (
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// readingOf is one note's place in the reading queue, or a failure.
func readingOf(t *testing.T, lib *store.Library, note core.ID) store.Reading {
	t.Helper()
	entry, ok := lib.ReadingFor(note)
	if !ok {
		t.Fatalf("note %q is not in the reading queue", note)
	}
	return entry
}

// The reading queue is plain text and replay: a session's difficulty and the
// block it stopped at are log lines, and an index thrown away and derived again
// answers the same.
func TestReadingRoundTrip(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first paragraph", "second paragraph", "third paragraph")
	position := note.Blocks[1].ID

	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	clk.advance(time.Hour)
	if err := lib.ReadNote(note.ID, schedule.Good, position, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}

	// The log is the record, and it is readable by hand: plain JSON lines.
	log := string(read(t, dir+"/reviews.jsonl"))
	for _, want := range []string{`"note":"` + string(note.ID) + `"`, `"kind":"queued"`, `"position":"` + string(position) + `"`, `"rating":"good"`} {
		if !strings.Contains(log, want) {
			t.Errorf("reviews.jsonl does not hold %s:\n%s", want, log)
		}
	}

	before := readingOf(t, lib, note.ID)
	if !before.Queued {
		t.Error("a read note left the reading queue")
	}
	if before.Position != position {
		t.Errorf("position = %q, want %q", before.Position, position)
	}
	if before.State.Reviews != 1 {
		t.Errorf("reviews = %d, want 1", before.State.Reviews)
	}

	// Copying the library to another machine is opening it there: the position
	// and the cadence are in the log, not in the index.
	reopened := reopen(t, lib, dir, clk)
	after := readingOf(t, reopened, note.ID)
	if !sameState(before.State, after.State) || after.Position != before.Position {
		t.Errorf("reopening changed the entry: %+v, want %+v", after, before)
	}

	// The index is a cache: deleting it changes nothing.
	if err := reopened.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	rebuilt := readingOf(t, reopened, note.ID)
	if !sameState(after.State, rebuilt.State) || rebuilt.Position != after.Position || !rebuilt.Queued {
		t.Errorf("the rebuild changed the entry: %+v, want %+v", rebuilt, after)
	}
}

// Membership is the log's: a note leaves the queue with an event, keeps the
// place it stopped at, and comes back where it left off.
func TestReadingQueueIsMembership(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first", "second")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	// Adding twice is one event, so the same note is not queued twice.
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading again: %v", err)
	}
	if queued, _ := lib.ReadingQueue(); len(queued) != 1 {
		t.Fatalf("reading queue = %d entries, want 1", len(queued))
	}

	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if err := lib.RemoveFromReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("RemoveFromReading: %v", err)
	}
	queued, err := lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	if len(queued) != 0 {
		t.Fatalf("reading queue = %d entries after a removal, want 0", len(queued))
	}
	dropped := readingOf(t, lib, note.ID)
	if dropped.Queued {
		t.Error("the entry is still queued after it was removed")
	}
	if dropped.Position != note.Blocks[0].ID {
		t.Error("a removal took the reading position with it")
	}

	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading back: %v", err)
	}
	back := readingOf(t, lib, note.ID)
	if !back.Queued || back.Position != dropped.Position {
		t.Errorf("adding back gave %+v, want it queued at %q", back, dropped.Position)
	}
}

// The three difficulties a reader can give are FSRS ratings, so the cadence
// follows the reading and not a fixed interval.
func TestReadingDifficultyMovesTheDueDate(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "para")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	entries, err := lib.ReadingQueue()
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadingQueue = %v, %v", entries, err)
	}
	if !entries[0].Due().Equal(clk.Now()) {
		t.Errorf("a note never read is due at %v, want the moment it joined (%v)", entries[0].Due(), clk.Now())
	}

	for _, rating := range []schedule.Rating{schedule.Again, schedule.Good, schedule.Easy} {
		if err := lib.ReadNote(note.ID, rating, note.Blocks[0].ID, clk.Now()); err != nil {
			t.Fatalf("ReadNote(%v): %v", rating, err)
		}
		entry := readingOf(t, lib, note.ID)
		if !entry.State.Due.After(clk.Now()) {
			t.Errorf("rating %v due at %v, want after now", rating, entry.State.Due)
		}
		if entry.State.LastRating != rating {
			t.Errorf("last rating = %v, want %v", entry.State.LastRating, rating)
		}
	}
}

// A reading session on a note that was never queued puts it in the queue: a
// note someone read is a note they mean to come back to.
func TestReadingUnqueuedNoteJoins(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "para")
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	queued, err := lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	if len(queued) != 1 || queued[0].Note != note.ID {
		t.Fatalf("reading queue = %+v, want the note that was read", queued)
	}
}

// An excerpt is a child note: the passage as its first block, and a reference
// back to the block it was taken from, which the source's backlinks find.
func TestExcerptMakesAChildNote(t *testing.T) {
	lib, _, _ := newLibrary(t)
	source := saveNote(t, lib, "Source note", "A paragraph worth keeping.", "Another paragraph.")

	child, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "A paragraph worth keeping.", "Excerpt of Source note")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	if child.ID == source.ID || child.ID.Kind() != "note" {
		t.Fatalf("the excerpt is not a child note: %+v", child)
	}
	if len(child.Blocks) != 1 {
		t.Fatalf("the excerpt has %d blocks, want 1", len(child.Blocks))
	}
	block := child.Blocks[0]
	if block.ID == "" {
		t.Error("the excerpt block has no ID, so no card can point at it")
	}
	if !strings.HasPrefix(block.Text, "A paragraph worth keeping.") {
		t.Errorf("the excerpt does not carry the passage: %q", block.Text)
	}
	links := core.Links(block.Text)
	if len(links) != 1 {
		t.Fatalf("the excerpt carries %d links, want 1: %q", len(links), block.Text)
	}
	if links[0].Title != "Source note" || links[0].Block != source.Blocks[0].ID {
		t.Errorf("the reference is %+v, want Source note at %q", links[0], source.Blocks[0].ID)
	}

	backlinks := lib.Backlinks(source.ID)
	if len(backlinks) != 1 || backlinks[0].Note != child.ID {
		t.Fatalf("the source's backlinks = %+v, want the excerpt", backlinks)
	}
}

// An excerpt of nothing is a mistake worth reporting, not an empty note.
func TestExcerptOfNothing(t *testing.T) {
	lib, _, _ := newLibrary(t)
	source := saveNote(t, lib, "Source note", "A paragraph.")
	if _, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "   ", "empty"); err == nil {
		t.Fatal("an empty excerpt was written")
	}
	if _, err := lib.Excerpt(source.ID, "block_missing", "text", "missing"); err == nil {
		t.Fatal("an excerpt of a missing block was written")
	}
}

// A reading event reaches the same handler a review does: a plugin watching the
// library sees the session and the queue move without a hook of its own.
func TestReadingEventsReachTheHandler(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "para")

	var seen []string
	lib.SetEventHandler(func(ev store.Event) {
		if ev.ID == note.ID {
			seen = append(seen, ev.Kind)
		}
	})
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if err := lib.RemoveFromReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("RemoveFromReading: %v", err)
	}
	want := []string{store.EventReadingQueued, store.EventReadingRecorded, store.EventReadingDropped}
	if len(seen) != len(want) {
		t.Fatalf("the handler saw %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("event %d is %q, want %q", i, seen[i], want[i])
		}
	}
}
