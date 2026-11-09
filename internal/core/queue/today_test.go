package queue

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// todayNote saves a note of paragraphs blocks, so a reading session can stop at
// any one of them.
func todayNote(t *testing.T, lib *store.Library, title string, paragraphs int) core.Note {
	t.Helper()
	blocks := make([]core.Block, paragraphs)
	for i := range blocks {
		blocks[i] = core.Block{Text: fmt.Sprintf("paragraph %d", i+1)}
	}
	note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	return note
}

// queueNote puts a note in the reading queue. A note no session has read is due
// from the moment it joins.
func queueNote(t *testing.T, lib *store.Library, note core.Note, at time.Time) {
	t.Helper()
	if err := lib.AddToReading(note.ID, at); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
}

// stopReading records a reading session that stopped at the given paragraph,
// counted from one.
func stopReading(t *testing.T, lib *store.Library, note core.Note, paragraph int, rating schedule.Rating, at time.Time) {
	t.Helper()
	if err := lib.ReadNote(note.ID, rating, note.Blocks[paragraph-1].ID, at); err != nil {
		t.Fatalf("read note: %v", err)
	}
}

// todayLines names each entry by its kind and, for a note, by its title: the order
// and the shape are what these tests freeze.
func todayLines(today Today) []string {
	kinds := [...]string{"read", "excerpt", "review"}
	out := make([]string, 0, len(today.Entries))
	for _, entry := range today.Entries {
		line := kinds[entry.Kind]
		if entry.Kind != EntryReview {
			line += " " + entry.Title
		}
		out = append(out, line)
	}
	return out
}

// Today is reading before recall: the notes due to read, then the notes read but
// not excerpted — most recently read first — and the day's cards as one line at
// the end, whatever the notes say.
func TestTodayOrdersReadingThenExcerptsThenReview(t *testing.T) {
	lib := library(t)
	// Queued at three moments, never read: due in the order they joined.
	for i, title := range []string{"oldest", "middle", "newest"} {
		queueNote(t, lib, todayNote(t, lib, title, 2), now.AddDate(0, 0, i-3))
	}
	// Read, so not due again today: they come back as excerpts, the fresher
	// passage first.
	stopReading(t, lib, todayNote(t, lib, "read first", 3), 2, schedule.Good, now.Add(-2*time.Hour))
	stopReading(t, lib, todayNote(t, lib, "read second", 4), 3, schedule.Good, now.Add(-time.Hour))
	// One card due, for the line that always comes last.
	cards(t, lib, 1)

	today, err := Entries(lib, now, TodayOptions{})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	want := []string{"read oldest", "read middle", "read newest",
		"excerpt read second", "excerpt read first", "review"}
	if got := todayLines(today); !slices.Equal(got, want) {
		t.Fatalf("today = %v, want %v", got, want)
	}
	// The order is the queue's, not the call's: asking twice answers the same.
	again, err := Entries(lib, now, TodayOptions{})
	if err != nil {
		t.Fatalf("Entries twice: %v", err)
	}
	if got := todayLines(again); !slices.Equal(got, want) {
		t.Errorf("today changed between calls: %v, want %v", got, want)
	}
}

// A reading line says where the note stands: how many paragraphs it holds and
// which one the last session stopped at.
func TestTodayShowsHowFarIntoANoteTheReaderGot(t *testing.T) {
	lib := library(t)
	note := todayNote(t, lib, "long", 6)
	queueNote(t, lib, note, now.AddDate(0, 0, -1))
	stopReading(t, lib, note, 2, schedule.Again, now.AddDate(0, 0, -2))

	today, err := Entries(lib, now, TodayOptions{})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	read := today.Entries[0]
	if read.Kind != EntryRead || read.Title != "long" {
		t.Fatalf("first entry = %+v, want the note due to read", read)
	}
	if read.Blocks != 6 || read.Read != 2 {
		t.Errorf("entry is at %d of %d, want 2 of 6", read.Read, read.Blocks)
	}
	if read.Position != note.Blocks[1].ID {
		t.Errorf("position = %q, want the second paragraph", read.Position)
	}
}

// The review line is one line for every card due, and it leaves the day when no
// card is: a queue that shows a review nothing to review is a lie.
func TestTodayListsReviewOnlyWhileCardsRemain(t *testing.T) {
	lib := library(t)
	reviewable := cards(t, lib, 2)

	today, err := Entries(lib, now, TodayOptions{})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(today.Entries) != 1 || today.Entries[0].Kind != EntryReview {
		t.Fatalf("today = %v, want the review line alone", todayLines(today))
	}
	if review := today.Entries[0]; review.Cards != 2 || review.New != 2 {
		t.Errorf("review line = %d cards, %d new, want 2 and 2", review.Cards, review.New)
	}

	// Suspending both takes them out of the queue, and the line with them.
	for _, card := range reviewable {
		card.Status = core.StatusSuspended
		if err := lib.SaveCard(card); err != nil {
			t.Fatalf("save card: %v", err)
		}
	}
	today, err = Entries(lib, now, TodayOptions{})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(today.Entries) != 0 {
		t.Fatalf("today = %v, want nothing left to review", todayLines(today))
	}
}

// The budget is where the day runs out: the line that cannot be paid for is
// marked, and so is everything after it — an entry is past the budget for where
// it falls, not for how long it is. A line landing exactly on the budget is not
// past it, and a budget of nothing truncates nothing.
func TestTodayBudgetStartsWhereTheDayRunsOut(t *testing.T) {
	for _, tt := range []struct {
		name     string
		budget   time.Duration
		overFrom int // the first entry past the budget; -1 when none is
	}{
		{"nothing budgeted", 0, -1},
		{"met exactly", 30 * time.Minute, -1},
		{"a minute short", 29 * time.Minute, 4},
		{"less than one line", 5 * time.Minute, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib := library(t)
			// Five notes of one paragraph, six minutes apiece.
			for i := range 5 {
				queueNote(t, lib, todayNote(t, lib, fmt.Sprintf("n%d", i), 1), now.AddDate(0, 0, i-5))
			}

			today, err := Entries(lib, now, TodayOptions{Budget: tt.budget})
			if err != nil {
				t.Fatalf("Entries: %v", err)
			}
			if len(today.Entries) != 5 {
				t.Fatalf("today = %v, want the five notes", todayLines(today))
			}
			for i, entry := range today.Entries {
				want := tt.overFrom >= 0 && i >= tt.overFrom
				if entry.Over != want {
					t.Errorf("entry %d over = %v, want %v", i, entry.Over, want)
				}
			}
			if today.ReadMinutes != 30 || today.TotalMinutes != 30 {
				t.Errorf("minutes = %d read, %d total, want 30 and 30", today.ReadMinutes, today.TotalMinutes)
			}
			if today.Budget != int(tt.budget/time.Minute) {
				t.Errorf("budget = %d minutes, want %d", today.Budget, int(tt.budget/time.Minute))
			}
		})
	}
}

// Reviewing is what the day has left: its minutes count towards the total, but
// the reading budget never pushes the review line past itself — a card queue is
// not postponed the way a reading is.
func TestTodayPricesTheReviewLineApart(t *testing.T) {
	lib := library(t)
	for i := range 5 {
		queueNote(t, lib, todayNote(t, lib, fmt.Sprintf("n%d", i), 1), now.AddDate(0, 0, i-5))
	}
	cards(t, lib, 19)

	today, err := Entries(lib, now, TodayOptions{Budget: 30 * time.Minute})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	for _, entry := range today.Entries[:5] {
		if entry.Over {
			t.Errorf("a reading line starting at %s is over a budget of 30", entry.Title)
		}
	}
	review := today.Entries[5]
	if review.Kind != EntryReview || review.Over {
		t.Fatalf("last entry = %+v, want the review line, inside the budget", review)
	}
	if review.Cards != 19 || review.Minutes != 11 {
		t.Errorf("review line = %d cards in %d minutes, want 19 in 11", review.Cards, review.Minutes)
	}
	if today.ReadMinutes != 30 || today.ReviewMinutes != 11 || today.TotalMinutes != 41 {
		t.Errorf("minutes = %d read, %d review, %d total, want 30, 11 and 41",
			today.ReadMinutes, today.ReviewMinutes, today.TotalMinutes)
	}
}

// A day offers a couple of excerpts and no more: two by default, as many as the
// caller asks for, and never a line for a passage there is nothing to keep.
func TestTodayOffersSoManyExcerpts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		slots int
		want  int
	}{
		{"the default", 0, 2},
		{"one", 1, 1},
		{"more than it has", 5, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib := library(t)
			for i := range 3 {
				note := todayNote(t, lib, fmt.Sprintf("r%d", i), 2)
				stopReading(t, lib, note, 1, schedule.Good, now.Add(-time.Duration(i+1)*time.Hour))
			}

			today, err := Entries(lib, now, TodayOptions{ExcerptSlots: tt.slots})
			if err != nil {
				t.Fatalf("Entries: %v", err)
			}
			if len(today.Entries) != tt.want {
				t.Fatalf("today = %v, want %d excerpts", todayLines(today), tt.want)
			}
			for i, entry := range today.Entries {
				if want := fmt.Sprintf("r%d", i); entry.Kind != EntryExcerpt || entry.Title != want {
					t.Errorf("entry %d = %s %q, want the excerpt of %q", i, todayLines(today)[i], entry.Title, want)
				}
			}
		})
	}
}

// One note is one line: a note due to read and still owing an excerpt is read
// first, and the excerpt slot it would have taken goes to the next note that
// owes one.
func TestTodayListsANoteOnce(t *testing.T) {
	lib := library(t)
	// Read two days ago and answered again, so it is due now.
	both := todayNote(t, lib, "both", 2)
	stopReading(t, lib, both, 2, schedule.Again, now.AddDate(0, 0, -2))
	// Read an hour ago: not due, but the passage is still owed.
	stopReading(t, lib, todayNote(t, lib, "owes", 2), 1, schedule.Good, now.Add(-time.Hour))
	// Read three days ago and answered easily, so it waits for its next reading.
	stopReading(t, lib, todayNote(t, lib, "waits", 2), 2, schedule.Easy, now.AddDate(0, 0, -3))

	today, err := Entries(lib, now, TodayOptions{ExcerptSlots: 2})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	want := []string{"read both", "excerpt owes", "excerpt waits"}
	if got := todayLines(today); !slices.Equal(got, want) {
		t.Fatalf("today = %v, want %v", got, want)
	}
}

// An empty library is an empty day, not an error: no entries, no minutes spent,
// and the budget still read back as it was given.
func TestTodayOfAnEmptyLibrary(t *testing.T) {
	lib := library(t)

	today, err := Entries(lib, now, TodayOptions{Budget: 30 * time.Minute})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(today.Entries) != 0 {
		t.Fatalf("today = %v, want nothing to do", todayLines(today))
	}
	if today.ReadMinutes != 0 || today.ReviewMinutes != 0 || today.TotalMinutes != 0 {
		t.Errorf("minutes = %+v, want none spent", today)
	}
	if today.Budget != 30 {
		t.Errorf("budget = %d minutes, want 30", today.Budget)
	}
}

// Estimate prices a sitting from the same two numbers the budget is made of, so
// a screen showing a group of cards knows what it costs before it opens them.
func TestEstimatePricesASitting(t *testing.T) {
	// Two readings and nineteen cards: twelve minutes of reading, eleven and a
	// half of recall.
	if got, want := Estimate(2, 19).Round(time.Second), 23*time.Minute+24*time.Second; got != want {
		t.Errorf("Estimate(2, 19) = %v, want %v", got, want)
	}
	if got := Estimate(0, 0); got != 0 {
		t.Errorf("Estimate(0, 0) = %v, want nothing", got)
	}
}
