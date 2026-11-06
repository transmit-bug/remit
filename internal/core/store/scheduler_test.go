package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// scheduledLibrary writes a library whose reviews are stamped with the scheduler
// given.
func scheduledLibrary(t *testing.T, scheduler string) (*store.Library, string, core.Card) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{Scheduler: scheduler, Now: func() time.Time {
		return time.Date(2026, time.November, 7, 9, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatal(err)
	}
	card, err := lib.CreateCard(note.Blocks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return lib, dir, card
}

// Every review carries what scheduled it: the family and version, in the log
// line a reader and a later auditor see.
func TestReviewsCarryTheirScheduler(t *testing.T) {
	lib, dir, card := scheduledLibrary(t, "fsrs-1")
	if err := lib.Review(card.ID, schedule.Good, time.Date(2026, time.November, 7, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	events := lib.Events()
	if len(events) != 1 {
		t.Fatalf("the log holds %d events, want 1", len(events))
	}
	if events[0].Scheduler != "fsrs-1" {
		t.Errorf("the review is stamped %q, want fsrs-1", events[0].Scheduler)
	}
	if got := lib.Scheduler(); got != "fsrs-1" {
		t.Errorf("the library schedules with %q", got)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}

	// The line on disk says so, not only the in-memory event.
	path := filepath.Join(dir, "reviews.jsonl")
	line, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if !strings.Contains(string(line), `"scheduler":"fsrs-1"`) {
		t.Errorf("the log line does not name its scheduler:\n%s", line)
	}
}

// A review written before logs carried a name reads as one that has none, rather
// than refusing to load.
func TestReviewsWithoutASchedulerRead(t *testing.T) {
	lib, dir, card := scheduledLibrary(t, "")
	if err := lib.Review(card.ID, schedule.Good, time.Date(2026, time.November, 7, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	events := again.Events()
	if len(events) != 1 || events[0].Scheduler != "" {
		t.Fatalf("the unnamed review read as %+v", events)
	}
}

// Switching the scheduler leaves the history alone: what was recorded keeps its
// own name, and the schedule it left is not recomputed.
func TestSwitchingSchedulersKeepsHistory(t *testing.T) {
	lib, dir, card := scheduledLibrary(t, "fsrs-1")
	at := time.Date(2026, time.November, 7, 10, 0, 0, 0, time.UTC)
	if err := lib.Review(card.ID, schedule.Good, at); err != nil {
		t.Fatal(err)
	}
	before, ok := lib.CardState(card.ID)
	if !ok {
		t.Fatal("the card has no state")
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := store.Open(dir, store.Options{Scheduler: "sm2-2"})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()

	after, ok := again.CardState(card.ID)
	if !ok {
		t.Fatal("the card lost its state")
	}
	if after.Reviews != before.Reviews || !after.Due.Equal(before.Due) || after.Memory != before.Memory {
		t.Errorf("the history was recomputed: %+v, then %+v", before, after)
	}
	events := again.Events()
	if len(events) != 1 || events[0].Scheduler != "fsrs-1" {
		t.Errorf("the recorded review changed its name: %+v", events)
	}
	if got := again.Scheduler(); got != "sm2-2" {
		t.Errorf("the library schedules with %q, want the new one", got)
	}
}
